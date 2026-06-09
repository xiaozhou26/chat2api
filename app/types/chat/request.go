package chat

type Author struct {
	Role string `json:"role"`
}

type Content struct {
	ContentType string        `json:"content_type"`
	Parts       []interface{} `json:"parts"`
}

type Attachment struct {
	ID       string `json:"id"`
	MimeType string `json:"mimeType"`
	Name     string `json:"name"`
	Size     int    `json:"size"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type Message struct {
	Id       string                 `json:"id"`
	Author   Author                 `json:"author"`
	Content  Content                `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ConversationMode struct {
	Kind string `json:"kind"`
}

type Request struct {
	Action                           string               `json:"action"`
	Messages                         []Message            `json:"messages"`
	ConversationId                   string               `json:"conversation_id,omitempty"`
	ParentMessageId                  string               `json:"parent_message_id"`
	Model                            string               `json:"model"`
	ClientPrepareState               string               `json:"client_prepare_state,omitempty"`
	Timezone                         string               `json:"timezone"`
	TimeZoneOffsetMin                int                  `json:"timezone_offset_min"`
	EnableMessageFollowups           bool                 `json:"enable_message_followups,omitempty"`
	Suggestions                      []string             `json:"suggestions"`
	SupportedEncodings               []string             `json:"supported_encodings"`
	SystemHints                      []string             `json:"system_hints"`
	SupportsBuffering                bool                 `json:"supports_buffering,omitempty"`
	HistoryAndTrainingDisabled       bool                 `json:"history_and_training_disabled"`
	ForceUseSse                      bool                 `json:"force_use_sse"`
	FaceUseSse                       bool                 `json:"face_use_sse"`
	ForceParagen                     bool                 `json:"force_paragen"`
	ForceParagenModelSlug            string               `json:"force_paragen_model_slug"`
	ForceRateLimit                   bool                 `json:"force_rate_limit"`
	ResetRateLimits                  bool                 `json:"reset_rate_limits"`
	VariantPurpose                   string               `json:"variant_purpose"`
	ConversationMode                 ConversationMode     `json:"conversation_mode"`
	WebsocketRequestId               string               `json:"websocket_request_id"`
	ClientContextualInfo             ClientContextualInfo `json:"client_contextual_info"`
	ParagenCotSummaryDisplayOverride string               `json:"paragen_cot_summary_display_override,omitempty"`
	ForceParallelSwitch              string               `json:"force_parallel_switch,omitempty"`
	ThinkingEffort                   string               `json:"thinking_effort,omitempty"`
}

type ClientContextualInfo struct {
	IsDarkMode      bool    `json:"is_dark_mode"`
	TimeSinceLoaded int     `json:"time_since_loaded"`
	PageHeight      int     `json:"page_height"`
	PageWidth       int     `json:"page_width"`
	PixelRatio      float64 `json:"pixel_ratio"`
	ScreenHeight    int     `json:"screen_height"`
	ScreenWidth     int     `json:"screen_width"`
	AppName         string  `json:"app_name,omitempty"`
}
