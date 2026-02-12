package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/local/picobot/internal/chat"
)

const (
	discordAPIBase   = "https://discord.com/api/v10"
	discordGatewayWS = "wss://gateway.discord.gg/?v=10&encoding=json"
	discordMaxLen    = 2000 // Discord message limit

	// Discord Gateway opcodes
	opDispatch       = 0
	opHeartbeat      = 1
	opIdentify       = 2
	opHeartbeatACK   = 11
	opHello          = 10
	opReconnect      = 7
	opInvalidSession = 9
)

// typingInterval is how often to re-trigger the typing indicator (Discord shows it for ~10s).
const typingInterval = 8 * time.Second

// splitContent splits content into chunks of at most maxLen runes (Discord limit).
func splitContent(content string, maxLen int) []string {
	r := []rune(content)
	if len(r) <= maxLen {
		return []string{content}
	}
	var chunks []string
	for i := 0; i < len(r); i += maxLen {
		end := i + maxLen
		if end > len(r) {
			end = len(r)
		}
		chunks = append(chunks, string(r[i:end]))
	}
	return chunks
}

// StartDiscord connects to the Discord Gateway, receives DM messages only,
// and forwards them to the hub. Replies are sent via the Discord REST API.
// allowFrom restricts which Discord user IDs may send messages. Empty means allow all.
func StartDiscord(ctx context.Context, hub *chat.Hub, token string, allowFrom []string) error {
	if token == "" {
		return fmt.Errorf("discord token not provided")
	}

	allowed := make(map[string]struct{}, len(allowFrom))
	for _, id := range allowFrom {
		allowed[id] = struct{}{}
	}

	// Tracks channels where we're "typing" (agent is processing).
	var typingMu sync.Mutex
	typingChannels := make(map[string]struct{})

	// Typing indicator goroutine: re-trigger every 8s for active channels.
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		ticker := time.NewTicker(typingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				typingMu.Lock()
				channels := make([]string, 0, len(typingChannels))
				for ch := range typingChannels {
					channels = append(channels, ch)
				}
				typingMu.Unlock()
				for _, chID := range channels {
					u := discordAPIBase + "/channels/" + chID + "/typing"
					req, _ := http.NewRequest("POST", u, nil)
					req.Header.Set("Authorization", "Bot "+token)
					resp, err := client.Do(req)
					if err != nil {
						continue
					}
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}
		}
	}()

	// Outbound sender goroutine
	go func() {
		client := &http.Client{Timeout: 10 * time.Second}
		for {
			select {
			case <-ctx.Done():
				log.Println("discord: stopping outbound sender")
				return
			case out := <-hub.Out:
				if out.Channel != "discord" {
					continue
				}
				// Stop typing indicator for this channel
				typingMu.Lock()
				delete(typingChannels, out.ChatID)
				typingMu.Unlock()
				u := discordAPIBase + "/channels/" + out.ChatID + "/messages"
				for _, chunk := range splitContent(out.Content, discordMaxLen) {
					body := map[string]interface{}{"content": chunk}
					b, _ := json.Marshal(body)
					req, err := http.NewRequest("POST", u, bytes.NewReader(b))
					if err != nil {
						log.Printf("discord sendMessage error: %v", err)
						continue
					}
					req.Header.Set("Authorization", "Bot "+token)
					req.Header.Set("Content-Type", "application/json")
					resp, err := client.Do(req)
					if err != nil {
						log.Printf("discord sendMessage error: %v", err)
						continue
					}
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					if resp.StatusCode >= 400 {
						log.Printf("discord sendMessage HTTP %d", resp.StatusCode)
						break
					}
				}
			}
		}
	}()

	// Gateway connection loop with reconnect
	go runGateway(ctx, hub, token, allowed, &typingMu, typingChannels)
	return nil
}

func runGateway(ctx context.Context, hub *chat.Hub, token string, allowed map[string]struct{}, typingMu *sync.Mutex, typingChannels map[string]struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := connectGateway(ctx, hub, token, allowed, typingMu, typingChannels)
		if err != nil && ctx.Err() == nil {
			log.Printf("discord gateway error: %v, reconnecting in 5s", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func connectGateway(ctx context.Context, hub *chat.Hub, token string, allowed map[string]struct{}, typingMu *sync.Mutex, typingChannels map[string]struct{}) error {
	conn, _, err := websocket.DefaultDialer.Dial(discordGatewayWS, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	var heartbeatInterval time.Duration
	var lastSeq *int64
	var mu sync.Mutex

	// Read loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var payload struct {
				Op int             `json:"op"`
				D  json.RawMessage `json:"d"`
				S  *int64          `json:"s"`
				T  string          `json:"t"`
			}
			if err := json.Unmarshal(raw, &payload); err != nil {
				continue
			}

			mu.Lock()
			if payload.S != nil {
				lastSeq = payload.S
			}
			mu.Unlock()

			switch payload.Op {
			case opHello:
				var hello struct {
					HeartbeatInterval int `json:"heartbeat_interval"`
				}
				if err := json.Unmarshal(payload.D, &hello); err != nil {
					continue
				}
				heartbeatInterval = time.Duration(hello.HeartbeatInterval) * time.Millisecond
			case opHeartbeat:
				sendHeartbeat(conn, lastSeq) // best-effort, ignore error
			case opHeartbeatACK:
				// no-op
			case opInvalidSession:
				// Session invalidated; runGateway will reconnect
			case opReconnect:
				// Server requested reconnect; connection will close shortly
			case opDispatch:
				if payload.T == "READY" {
					log.Println("discord: connected and ready")
				}
				if payload.T == "MESSAGE_CREATE" {
					handleMessageCreate(payload.D, hub, token, allowed, typingMu, typingChannels)
				}
			}
		}
	}()

	// Wait for Hello to get heartbeat interval
	time.Sleep(2 * time.Second)
	if heartbeatInterval == 0 {
		heartbeatInterval = 45 * time.Second
	}

	// Send Identify
	identify := map[string]interface{}{
		"op": 2,
		"d": map[string]interface{}{
			"token": token,
			"properties": map[string]string{
				"os":      "linux",
				"browser": "picobot",
				"device":  "picobot",
			},
			"intents": 1<<12 | 1<<15, // DIRECT_MESSAGES | MESSAGE_CONTENT
		},
	}
	if err := conn.WriteJSON(identify); err != nil {
		return err
	}

	// Heartbeat loop
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(1000, "shutdown"), time.Now().Add(time.Second))
			return nil
		case <-ticker.C:
			mu.Lock()
			seq := lastSeq
			mu.Unlock()
			if err := sendHeartbeat(conn, seq); err != nil {
				return err
			}
		}
	}
}

func triggerTyping(channelID, token string) {
	u := discordAPIBase + "/channels/" + channelID + "/typing"
	req, err := http.NewRequest("POST", u, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bot "+token)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func sendHeartbeat(conn *websocket.Conn, seq *int64) error {
	payload := map[string]interface{}{"op": opHeartbeat, "d": nil}
	if seq != nil {
		payload["d"] = *seq
	}
	return conn.WriteJSON(payload)
}

func handleMessageCreate(d json.RawMessage, hub *chat.Hub, token string, allowed map[string]struct{}, typingMu *sync.Mutex, typingChannels map[string]struct{}) {
	var msg struct {
		ChannelID   string `json:"channel_id"`
		Content      string `json:"content"`
		GuildID      string `json:"guild_id"`
		Attachments []discordAttachment `json:"attachments"`
		Author *struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Bot      bool   `json:"bot"`
		} `json:"author"`
	}
	if err := json.Unmarshal(d, &msg); err != nil {
		return
	}
	// Only process DMs: guild_id empty and not from a bot
	if msg.GuildID != "" {
		return
	}
	if msg.Author != nil && msg.Author.Bot {
		return
	}
	fromID := ""
	if msg.Author != nil {
		fromID = msg.Author.ID
	}
	if fromID == "" {
		return
	}
	if len(allowed) > 0 {
		if _, ok := allowed[fromID]; !ok {
			log.Printf("discord: dropping message from unauthorized user %s", fromID)
			return
		}
	}
	// Show typing indicator while agent processes
	typingMu.Lock()
	typingChannels[msg.ChannelID] = struct{}{}
	typingMu.Unlock()
	// Trigger typing immediately (don't wait for the 8s ticker)
	go triggerTyping(msg.ChannelID, token)
	content, media := processAttachments(msg.Content, msg.Attachments)
	hub.In <- chat.Inbound{
		Channel:   "discord",
		SenderID:  fromID,
		ChatID:    msg.ChannelID,
		Content:   content,
		Media:     media,
		Timestamp: time.Now(),
	}
}

type discordAttachment struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Filename   string `json:"filename"`
}

// processAttachments returns (content, imageURLs). It fetches text files and appends to content,
// and collects image URLs for vision models.
func processAttachments(content string, attachments []discordAttachment) (string, []string) {
	var imageURLs []string
	imageTypes := map[string]bool{
		"image/png": true, "image/jpeg": true, "image/jpg": true,
		"image/gif": true, "image/webp": true,
	}
	textTypes := map[string]bool{
		"text/plain": true, "text/markdown": true, "text/csv": true,
		"application/json": true,
	}
	textExts := map[string]bool{
		".txt": true, ".md": true, ".csv": true, ".json": true,
	}
	imageExts := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	}

	var textParts []string
	if strings.TrimSpace(content) != "" {
		textParts = append(textParts, content)
	}

	for _, a := range attachments {
		if a.URL == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(a.Filename))
		isImage := imageTypes[a.ContentType] || (a.ContentType == "" && imageExts[ext])
		isText := textTypes[a.ContentType] || (a.ContentType == "" && textExts[ext])

		if isImage {
			imageURLs = append(imageURLs, a.URL)
		} else if isText {
			body, err := fetchURL(a.URL)
			if err == nil && len(body) > 0 {
				textParts = append(textParts, fmt.Sprintf("[Attachment: %s]\n%s", a.Filename, body))
			}
		}
	}

	finalContent := strings.Join(textParts, "\n\n")
	if finalContent == "" && len(imageURLs) > 0 {
		finalContent = "[Image(s) attached]"
	}
	return finalContent, imageURLs
}

func fetchURL(url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	// Limit size to avoid huge payloads (e.g. 500KB)
	if len(body) > 500*1024 {
		return string(body[:500*1024]) + "\n\n[truncated...]", nil
	}
	return string(body), nil
}
