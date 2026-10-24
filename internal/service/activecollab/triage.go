package activecollab

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

// TriagePayload is posted to the Hermes webhook route that runs the Claude analysis
// of a new task and delivers it to the Telegram forum.
type TriagePayload struct {
	EventType      string `json:"event_type"`
	NotificationID int64  `json:"notification_id"`
	Class          string `json:"class"`
	TaskID         int64  `json:"task_id"`
	ProjectID      int64  `json:"project_id"`
	TaskName       string `json:"task_name"`
	URL            string `json:"url"`
}

// EnableTriage forwards new and reassigned task events to a Hermes webhook route.
// An empty URL keeps the service notification-only.
func (s *Service) EnableTriage(url, secret string) {
	s.triageURL = url
	s.triageSecret = secret
	s.triageHTTP = &http.Client{Timeout: 30 * time.Second}
}

func isTriageEvent(n Notification) bool {
	return n.ParentType == "Task" &&
		(n.Class == "NewTaskNotification" || n.Class == "TaskReassignedNotification")
}

func (s *Service) triagePayload(n Notification, related RelatedEntities) TriagePayload {
	p := TriagePayload{
		EventType:      "collab_task",
		NotificationID: n.ID,
		Class:          n.Class,
		TaskID:         n.ParentID,
	}
	if task, ok := related.Task[strconv.FormatInt(n.ParentID, 10)]; ok {
		p.ProjectID = task.ProjectID
		p.TaskName = task.Name
		if task.URLPath != "" {
			p.URL = s.baseURL + task.URLPath
		}
	}
	return p
}

// dispatchTriage retries only transport errors and 5xx: Hermes answers 200 "ignored" when the
// route script skips an event on purpose (duplicate task, task not assigned to the token owner).
func (s *Service) dispatchTriage(p TriagePayload) {
	body, err := json.Marshal(p)
	if err != nil {
		log.Printf("[ActiveCollab] Triage payload for task #%d not encodable: %v", p.TaskID, err)
		return
	}
	for attempt := 1; attempt <= 3; attempt++ {
		status, respBody, err := s.postTriage(body, p.NotificationID)
		if err == nil && status < 500 {
			log.Printf("[ActiveCollab] Triage task #%d sent: HTTP %d %s", p.TaskID, status, respBody)
			return
		}
		log.Printf("[ActiveCollab] Triage task #%d attempt %d failed: HTTP %d %v", p.TaskID, attempt, status, err)
		time.Sleep(time.Duration(attempt*10) * time.Second)
	}
}

// postTriage signs with Hermes' generic V2 scheme: X-Webhook-Signature-V2 is the hex
// HMAC-SHA256 of "<timestamp>.<body>", and the timestamp must be within 5 minutes of the server.
func (s *Service) postTriage(body []byte, notificationID int64) (int, string, error) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(s.triageSecret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)

	req, err := http.NewRequest(http.MethodPost, s.triageURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Timestamp", ts)
	req.Header.Set("X-Webhook-Signature-V2", hex.EncodeToString(mac.Sum(nil)))
	// Same notification -> same request id, so Hermes drops our own retries as duplicates.
	req.Header.Set("X-Request-ID", fmt.Sprintf("ac-notification-%d", notificationID))

	resp, err := s.triageHTTP.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	return resp.StatusCode, string(respBody), nil
}
