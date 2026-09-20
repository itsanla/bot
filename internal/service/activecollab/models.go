package activecollab

// Notification represents an item from ActiveCollab /api/v1/notifications.
type Notification struct {
	ID         int64  `json:"id"`
	Class      string `json:"class"`
	URLPath    string `json:"url_path"`
	ParentType string `json:"parent_type"`
	ParentID   int64  `json:"parent_id"`
	SenderID   int64  `json:"sender_id"`
	CommentID  int64  `json:"comment_id"`
	CreatedOn  int64  `json:"created_on"`
}

type Task struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	ProjectID     int64  `json:"project_id"`
	TaskNumber    int    `json:"task_number"`
	URLPath       string `json:"url_path"`
	CreatedByName string `json:"created_by_name"`
	BodyPlainText string `json:"body_plain_text"`
}

type Project struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	URLPath string `json:"url_path"`
}

type Comment struct {
	ID            int64  `json:"id"`
	BodyPlainText string `json:"body_plain_text"`
	CreatedByName string `json:"created_by_name"`
	ParentType    string `json:"parent_type"`
	ParentID      int64  `json:"parent_id"`
	URLPath       string `json:"url_path"`
}

type User struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	Name        string `json:"name"`
	Email       string `json:"email"`
}

type RelatedEntities struct {
	Task    map[string]Task    `json:"Task"`
	Project map[string]Project `json:"Project"`
	Comment map[string]Comment `json:"Comment"`
	User    map[string]User    `json:"User"`
}

type NotificationResponse struct {
	Notifications []Notification  `json:"notifications"`
	Related       RelatedEntities `json:"related"`
}
