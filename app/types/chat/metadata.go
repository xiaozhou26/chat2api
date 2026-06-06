package chat

type DalleContent struct {
	AssetPointer string `json:"asset_pointer"`
	Metadata     struct {
		Dalle struct {
			Prompt string `json:"prompt"`
		} `json:"dalle"`
	} `json:"metadata"`
}

type Metadata struct {
	Timestamp     string         `json:"timestamp_"`
	Citations     []Citation     `json:"citations,omitempty"`
	MessageType   string         `json:"message_type"`
	FinishDetails *FinishDetails `json:"finish_details"`
	ModelSlug     string         `json:"model_slug"`

	GizmoId           interface{} `json:"gizmo_id"`
	DefaultModelSlug  string      `json:"default_model_slug"`
	Pad               string      `json:"pad"`
	ParentId          string      `json:"parent_id"`
	ModelSwitcherDeny []struct {
		Slug        string `json:"slug"`
		Context     string `json:"context"`
		Reason      string `json:"reason"`
		Description string `json:"description"`
	} `json:"model_switcher_deny"`
}

type Citation struct {
	Metadata CitaMeta `json:"metadata"`
	StartIx  int      `json:"start_ix"`
	EndIx    int      `json:"end_ix"`
}

type CitaMeta struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type FinishDetails struct {
	Type string `json:"types"`
	Stop string `json:"stop"`
}
