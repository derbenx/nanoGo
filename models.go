package main

import "time"

type SafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type Config struct {
	APIKeyPaid       string          `json:"api_key_paid"`
	APIKeyFree       string          `json:"api_key_free"`
	IsFreeModeImage  bool            `json:"is_free_mode_image"`
	IsFreeModeChat   bool            `json:"is_free_mode_chat"`
	ChatModelList    string          `json:"chat_model_list"`
	OutputDir        string          `json:"output_dir"`
	DefaultPrompt    string          `json:"default_prompt"`
	DefaultNegPrompt string          `json:"default_neg_prompt"`
	EncourageEdt     string          `json:"encourage_edt"`
	EncourageGen     string          `json:"encourage_gen"`
	Debug            bool            `json:"debug"`
	SafetySettings   []SafetySetting `json:"safety_settings"`
	Temperature      float32         `json:"temperature"`
	TopP             float32         `json:"top_p"`
	TopK             int             `json:"top_k"`
	MaxOutputTokens  int             `json:"max_output_tokens"`

	ModelNanoFlash   string `json:"model_nano_flash"`
	ModelNanoPro     string `json:"model_nano_pro"`
	ModelNano2       string `json:"model_nano_2"`
	ModelImagen      string `json:"model_imagen"`
	ModelImagenUltra string `json:"model_imagen_ultra"`

	// GUI State
	WindowWidth  float32 `json:"window_width"`
	WindowHeight float32 `json:"window_height"`
	IsMaximized  bool    `json:"is_maximized"`

	SplitOffsetMain float64 `json:"split_offset_main"`
	SplitOffsetLeft float64 `json:"split_offset_left"`
	SplitOffsetTop  float64 `json:"split_offset_top"`
	LogSplitOffset  float64 `json:"log_split_offset"`

	ChatMemoryEnabled   bool `json:"chat_memory_enabled"`
	ChatRememberInitial bool `json:"chat_remember_initial"`
	ChatMemorySlots     int  `json:"chat_memory_slots"`
	ChatSystemPrompt    string `json:"chat_system_prompt"`
	ReturnThought       bool   `json:"return_thought"`
}

type ImageInfo struct {
	ID        string
	FileName  string
	FullPath  string
	SizeMB    float64
	TaskCount int
	Selected  bool // For checkbox selection
	Width     int
	Height    int
}

type TaskInfo struct {
	ID             int     `json:"ID"`
	ImgIDs         string  `json:"ImgIDs"`
	Agent          string  `json:"Agent"`
	Size           string  `json:"Size"`
	Ratio          string  `json:"Ratio"`
	Status         string  `json:"Status"`
	Cost           float64 `json:"Cost"`
	Prompt         string  `json:"Prompt"`
	NegativePrompt string  `json:"NegativePrompt"`
	Format         string  `json:"Format"`
	Disabled       bool    `json:"Disabled"`
	SourcePath     string  `json:"SourcePath"`
	RunningCount   int     `json:"RunningCount"`
	LastSavedPath  string  `json:"LastSavedPath"`
	ReturnThought  bool    `json:"ReturnThought"`
}

type BatchJob struct {
	JobID       string
	Status      string
	SubmittedAt time.Time
	Progress    string
	IsFree      bool
}
