package chat

type Author struct {
	Role string `json:"role"`
}

type Content struct {
	ContentType string   `json:"content_type"`
	Parts       []string `json:"parts"`
}

type Message struct {
	Id      string  `json:"id"`
	Author  Author  `json:"author"`
	Content Content `json:"content"`
}

type ConversationMode struct {
	Kind string `json:"kind"`
}

type Request struct {
	Action                     string               `json:"action"`
	Messages                   []Message            `json:"messages"`
	ConversationId             string               `json:"conversation_id,omitempty"`
	ParentMessageId            string               `json:"parent_message_id"`
	Model                      string               `json:"model"`
	Timezone                   string               `json:"timezone"`
	TimeZoneOffsetMin          int                  `json:"timezone_offset_min"`
	Suggestions                []string             `json:"suggestions"`
	SupportedEncodings         []string             `json:"supported_encodings"`
	SystemHints                []string             `json:"system_hints"`
	HistoryAndTrainingDisabled bool                 `json:"history_and_training_disabled"`
	ForceUseSse                bool                 `json:"force_use_sse"`
	FaceUseSse                 bool                 `json:"face_use_sse"`
	ForceParagen               bool                 `json:"force_paragen"`
	ForceParagenModelSlug      string               `json:"force_paragen_model_slug"`
	ForceRateLimit             bool                 `json:"force_rate_limit"`
	ResetRateLimits            bool                 `json:"reset_rate_limits"`
	VariantPurpose             string               `json:"variant_purpose"`
	ConversationMode           ConversationMode     `json:"conversation_mode"`
	WebsocketRequestId         string               `json:"websocket_request_id"`
	ClientContextualInfo       ClientContextualInfo `json:"client_contextual_info"`
}

type ClientContextualInfo struct {
	IsDarkMode      bool    `json:"is_dark_mode"`
	TimeSinceLoaded int     `json:"time_since_loaded"`
	PageHeight      int     `json:"page_height"`
	PageWidth       int     `json:"page_width"`
	PixelRatio      float64 `json:"pixel_ratio"`
	ScreenHeight    int     `json:"screen_height"`
	ScreenWidth     int     `json:"screen_width"`
}
