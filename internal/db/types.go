package db

import "time"

type MandatoryChannel struct {
	ID            int64
	ChannelID     string
	Title         string
	URL           string
	RequiresCheck bool
	Active        bool
	CreatedAt     time.Time
}

type Post struct {
	ID        int64
	Title     string
	Text      string
	MediaID   string
	MediaKind string
	Buttons   []PostButton
	CreatedBy string
	CreatedAt time.Time
}

type PostButton struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

type TrackLink struct {
	ID                  int64
	Code                string
	Label               string
	CreatedByUserID     string
	CreatedAt           time.Time
	ArrivalsCount       int64
	GeneratedUsersCount int64
}

type BroadcastStat struct {
	ID            int64
	Date          time.Time
	Type          string
	Total         int64
	Success       int64
	Error         int64
	Active        bool
	Status        string
	StopRequested bool
	AdminChatID   string
	PayloadJSON   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	FinishedAt    *time.Time
}

type BroadcastTarget struct {
	BroadcastID int64
	UserID      string
	SortOrder   int64
	Status      string
	Error       string
	UpdatedAt   time.Time
}

type Task struct {
	ID                 int64
	Reward             int64
	Active             bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CompletedTotal     int64
	CompletedToday     int64
	CompletedYesterday int64
	Channels           []TaskChannel
}

type TaskChannel struct {
	ID            int64
	TaskID        int64
	ChannelID     string
	Title         string
	URL           string
	RequiresCheck bool
	SortOrder     int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type PaymentTransaction struct {
	ID             int64
	TransactionID  string
	UserID         string
	PlatformUserID string
	ProductKey     string
	ProductTitle   string
	Coins          int64
	Amount         int64
	Currency       string
	PaymentMethod  int64
	Status         string
	RedirectURL    string
	Rewarded       bool
	PaidAt         *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

const (
	IntegrationProviderDeepSeek = "deepseek"
	IntegrationProviderGemini   = "gemini"
)

type IntegrationToken struct {
	ID        int64
	Provider  string
	Token     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type BotStats struct {
	UniqueTotal              int64
	UniqueToday              int64
	UniqueYesterday          int64
	NewUsersToday            int64
	NewUsersYesterday        int64
	StartsToday              int64
	StartsYesterday          int64
	RefTotal                 int64
	RefToday                 int64
	RefYesterday             int64
	PostSentTotal            int64
	PostSentToday            int64
	PostSentYesterday        int64
	PostFailedToday          int64
	PostFailedYesterday      int64
	MandatoryRewardTotal     int64
	MandatoryRewardToday     int64
	MandatoryRewardYesterday int64
	TrackVisitsTotal         int64
	TrackVisitsToday         int64
	TrackVisitsYesterday     int64
	BalanceAddedTotal        int64
	BalanceAddedToday        int64
	BalanceAddedYesterday    int64
	BroadcastsTotal          int64
	BroadcastsActive         int64
}
