package activecollab

import (
	"context"
	"fmt"
	"html"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/itsanla/bot/internal/db"
	"github.com/itsanla/bot/internal/telegram"
)

type Service struct {
	client       *Client
	db           *db.DB
	tg           *telegram.Client
	interval     time.Duration
	baseURL      string
	quietStart   int
	quietEnd     int
	loc          *time.Location
	inQuietHours bool
}

func NewService(baseURL, token string, database *db.DB, tgClient *telegram.Client, interval time.Duration, quietStart, quietEnd int, timezone string) *Service {
	baseURL = strings.TrimRight(baseURL, "/")

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	return &Service{
		client:     NewClient(baseURL, token),
		db:         database,
		tg:         tgClient,
		interval:   interval,
		baseURL:    baseURL,
		quietStart: quietStart,
		quietEnd:   quietEnd,
		loc:        loc,
	}
}

func (s *Service) Name() string {
	return "activecollab"
}

func (s *Service) Interval() time.Duration {
	return s.interval
}

// IsInQuietHours returns true if current time falls within the sleep window (e.g. 23:00 - 06:00).
func (s *Service) IsInQuietHours() bool {
	if s.quietStart == s.quietEnd {
		return false
	}
	now := time.Now().In(s.loc)
	hour := now.Hour()
	if s.quietStart > s.quietEnd {
		// Overnight range, e.g. 23 to 6: 23, 0, 1, 2, 3, 4, 5
		return hour >= s.quietStart || hour < s.quietEnd
	}
	return hour >= s.quietStart && hour < s.quietEnd
}

func (s *Service) Run(ctx context.Context) error {
	// Quiet hours check: pause polling between 23:00 and 06:00
	if s.IsInQuietHours() {
		if !s.inQuietHours {
			s.inQuietHours = true
			now := time.Now().In(s.loc)
			log.Printf("[ActiveCollab] Entering quiet hours (%02d:00 - %02d:00 %s, current: %02d:%02d). Polling paused until morning.",
				s.quietStart, s.quietEnd, s.loc.String(), now.Hour(), now.Minute())
		}
		return nil
	}

	if s.inQuietHours {
		s.inQuietHours = false
		now := time.Now().In(s.loc)
		log.Printf("[ActiveCollab] Exiting quiet hours (current: %02d:%02d %s). Resuming active polling.",
			now.Hour(), now.Minute(), s.loc.String())
	}

	lastID, err := s.db.GetLastEventID(s.Name())
	if err != nil {
		return fmt.Errorf("failed to get last event id: %w", err)
	}

	payload, err := s.client.GetNotifications(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch notifications: %w", err)
	}

	notifications := payload.Notifications
	if len(notifications) == 0 {
		return nil
	}

	// First time initialization: record current latest ID so we don't spam historical alerts
	if lastID == 0 {
		var maxID int64
		for _, n := range notifications {
			if n.ID > maxID {
				maxID = n.ID
			}
		}
		if err := s.db.SetLastEventID(s.Name(), maxID); err != nil {
			return fmt.Errorf("failed to initialize last event id: %w", err)
		}

		initMsg := fmt.Sprintf(
			"🤖 <b>ActiveCollab Service Aktif!</b>\n\n"+
				"• Server: <code>%s</code>\n"+
				"• Memantau update baru mulai ID: <b>#%d</b>\n"+
				"• Interval polling: <b>%v</b>\n"+
				"• Jam hening: <b>%02d:00 - %02d:00 WIB</b> (Polling mati saat tidur)\n\n"+
				"Notifikasi task dan komentar akan otomatis masuk ke sini.",
			html.EscapeString(s.baseURL), maxID, s.interval, s.quietStart, s.quietEnd,
		)
		_ = s.tg.SendMessage(initMsg)
		log.Printf("[ActiveCollab] Initialized tracking starting from event #%d", maxID)
		return nil
	}

	// Filter new notifications
	var newEvents []Notification
	for _, n := range notifications {
		if n.ID > lastID {
			newEvents = append(newEvents, n)
		}
	}

	if len(newEvents) == 0 {
		return nil
	}

	// Sort chronological (oldest to newest)
	sort.Slice(newEvents, func(i, j int) bool {
		return newEvents[i].ID < newEvents[j].ID
	})

	log.Printf("[ActiveCollab] Found %d new notification(s) to process", len(newEvents))

	for _, n := range newEvents {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if already sent
		sent, err := s.db.IsNotificationSent(s.Name(), n.ID)
		if err != nil {
			log.Printf("[ActiveCollab] Failed to check dedup for #%d: %v", n.ID, err)
			continue
		}
		if sent {
			_ = s.db.SetLastEventID(s.Name(), n.ID)
			continue
		}

		msg, title := s.formatMessage(ctx, n, payload.Related)
		if err := s.tg.SendMessage(msg); err != nil {
			log.Printf("[ActiveCollab] Failed to send telegram message for #%d: %v", n.ID, err)
			return err
		}

		_ = s.db.RecordNotification(s.Name(), n.ID, n.Class, title, "")
		_ = s.db.SetLastEventID(s.Name(), n.ID)

		// Short pause to avoid Telegram rate limits if multiple events burst
		time.Sleep(300 * time.Millisecond)
	}

	return nil
}

func (s *Service) formatMessage(ctx context.Context, n Notification, related RelatedEntities) (string, string) {
	icon, actionText := getActionMeta(n.Class)

	// Resolve sender name
	senderName := s.client.GetUserName(ctx, n.SenderID)

	// Resolve target
	targetType := n.ParentType
	targetName := ""
	webURL := s.baseURL
	if n.URLPath != "" {
		webURL = fmt.Sprintf("%s%s", s.baseURL, n.URLPath)
	}

	parentIDStr := strconv.FormatInt(n.ParentID, 10)
	switch n.ParentType {
	case "Task":
		if task, ok := related.Task[parentIDStr]; ok {
			targetName = task.Name
			if task.URLPath != "" {
				webURL = fmt.Sprintf("%s%s", s.baseURL, task.URLPath)
			}
		}
	case "Project":
		if proj, ok := related.Project[parentIDStr]; ok {
			targetName = proj.Name
			if proj.URLPath != "" {
				webURL = fmt.Sprintf("%s%s", s.baseURL, proj.URLPath)
			}
		}
	}

	if targetName == "" {
		targetName = fmt.Sprintf("%s #%d", targetType, n.ParentID)
	}

	// Comment excerpt if present
	commentSnippet := ""
	if n.CommentID > 0 {
		commentIDStr := strconv.FormatInt(n.CommentID, 10)
		if comm, ok := related.Comment[commentIDStr]; ok && comm.BodyPlainText != "" {
			body := strings.TrimSpace(comm.BodyPlainText)
			if len(body) > 280 {
				body = body[:280] + "..."
			}
			commentSnippet = body
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s <b>[ActiveCollab] %s</b>\n\n", icon, actionText))
	sb.WriteString(fmt.Sprintf("👤 <b>Oleh:</b> %s\n", html.EscapeString(senderName)))
	sb.WriteString(fmt.Sprintf("📋 <b>%s:</b> %s\n", html.EscapeString(targetType), html.EscapeString(targetName)))

	if commentSnippet != "" {
		sb.WriteString(fmt.Sprintf("\n💬 <i>\"%s\"</i>\n", html.EscapeString(commentSnippet)))
	}

	sb.WriteString(fmt.Sprintf("\n🔗 <a href=\"%s\">Buka di ActiveCollab</a>", webURL))

	return sb.String(), targetName
}

func getActionMeta(cls string) (string, string) {
	switch cls {
	case "NewCommentNotification":
		return "💬", "Komentar Baru"
	case "NewTaskNotification":
		return "📌", "Task Baru"
	case "TaskReassignedNotification":
		return "🔄", "Task Dialihkan"
	case "NewSubtaskNotification":
		return "📝", "Subtask Baru"
	case "SubtaskReassignedNotification":
		return "🔄", "Subtask Dialihkan"
	case "NewProjectNotification":
		return "📁", "Project Baru"
	case "NewReactionNotification":
		return "👍", "Reaksi Baru"
	default:
		name := strings.TrimSuffix(cls, "Notification")
		return "🔔", name
	}
}
