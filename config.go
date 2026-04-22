package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const configFileName = "config.json"

func GetConfigPath() string {
	execPath, _ := os.Executable()
	return filepath.Join(filepath.Dir(execPath), configFileName)
}

func LoadConfig() (*Config, error) {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	var cfg Config
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	// Migration and Fill in defaults
	def := DefaultConfig()

	// Handle migration from old APIKey if APIKeyPaid is empty
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if cfg.APIKeyPaid == "" {
		if oldKey, ok := raw["api_key"].(string); ok && oldKey != "" {
			cfg.APIKeyPaid = oldKey
		}
	}

	if cfg.ChatModelList == "" {
		cfg.ChatModelList = def.ChatModelList
	}
	if cfg.DefaultPrompt == "" {
		cfg.DefaultPrompt = def.DefaultPrompt
	}
	if cfg.DefaultNegPrompt == "" {
		cfg.DefaultNegPrompt = def.DefaultNegPrompt
	}

	if cfg.SafetySettings == nil {
		cfg.SafetySettings = def.SafetySettings
	}
	if cfg.ModelNanoFlash == "" {
		cfg.ModelNanoFlash = def.ModelNanoFlash
	}
	if cfg.ModelNanoPro == "" {
		cfg.ModelNanoPro = def.ModelNanoPro
	}
	if cfg.ModelNano2 == "" {
		cfg.ModelNano2 = def.ModelNano2
	}
	if cfg.ModelImagen == "" {
		cfg.ModelImagen = def.ModelImagen
	}
	if cfg.ModelImagenUltra == "" {
		cfg.ModelImagenUltra = def.ModelImagenUltra
	}

	if cfg.WindowWidth <= 100 {
		cfg.WindowWidth = def.WindowWidth
	}
	if cfg.WindowHeight <= 100 {
		cfg.WindowHeight = def.WindowHeight
	}
	if cfg.LogSplitOffset <= 0 || cfg.LogSplitOffset >= 1 {
		cfg.LogSplitOffset = def.LogSplitOffset
	}
	if cfg.SplitOffsetMain <= 0 || cfg.SplitOffsetMain >= 1 {
		cfg.SplitOffsetMain = def.SplitOffsetMain
	}

	if cfg.ChatSystemPrompt == "" {
		cfg.ChatSystemPrompt = def.ChatSystemPrompt
	}

	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	path := GetConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func DefaultConfig() *Config {
	chatModels := []string{
		"gemini-2.5-flash",
		"gemini-2.5-pro",
		"gemini-2.0-flash",
		"gemini-2.0-flash-001",
		"gemini-2.0-flash-lite-001",
		"gemini-2.0-flash-lite",
		"gemma-3-1b-it",
		"gemma-3-4b-it",
		"gemma-3-12b-it",
		"gemma-3-27b-it",
		"gemma-3n-e4b-it",
		"gemma-3n-e2b-it",
		"gemma-4-26b-a4b-it",
		"gemma-4-31b-it",
		"gemini-flash-latest",
		"gemini-flash-lite-latest",
		"gemini-pro-latest",
		"gemini-2.5-flash-lite",
		"gemini-3-pro-preview",
		"gemini-3-flash-preview",
		"gemini-3.1-pro-preview",
		"gemini-3.1-pro-preview-customtools",
		"gemini-3.1-flash-lite-preview",
	}

	return &Config{
		ChatModelList:    strings.Join(chatModels, "\n"),
		IsFreeModeImage:  false,
		IsFreeModeChat:   false,
		OutputDir:        "img",
		DefaultPrompt:    "everyone stands on top of a large pile of burgers. the burgers deform under load.",
		DefaultNegPrompt: "distorted faces, blurry, distortion, low quality, watermarks, missing limbs, extra limbs, deformities, floating people or objects, cropping body parts",
		EncourageEdt:     "You are a professional image-restoration engine. Please apply the 'USER DIRECTIVE' while maintaining strict structural integrity. Focus on high-fidelity surface rendering and cinematic lighting. Ensure all facial features are sharp, clear and perfectly aligned with the reference without looking plastic. Resolve blur into crisp, clean, 8k-resolution details. Maintain 100% adherence to the subject's identity. If the directive involves clothing, ensure the new attire is rendered with realistic fabric textures and consistent coverage. Your reasoning should always lead to an image generation call.",
		EncourageGen:     "You are a world-class visual concept artist. Please transform the user's prompt into a vivid, high-fidelity masterpiece. Prioritize cinematic lighting, photorealistic textures, and perfect anatomical detail. Every output must be rendered with the clarity of an 8k digital sensor. Interpret abstract concepts as concrete, visually dense scenes. Ensure all subjects, especially faces and hands, are rendered with sharp focus and professional-grade definition. Your reasoning should always lead to an image generation call.",
		Debug: false,
		SafetySettings: []SafetySetting{
			{"HARM_CATEGORY_HARASSMENT", "BLOCK_NONE"},
			{"HARM_CATEGORY_HATE_SPEECH", "BLOCK_NONE"},
			{"HARM_CATEGORY_SEXUALLY_EXPLICIT", "BLOCK_NONE"},
			{"HARM_CATEGORY_DANGEROUS_CONTENT", "BLOCK_NONE"},
		},
		Temperature:     1.0,
		TopP:            0.95,
		TopK:            40,
		MaxOutputTokens: 8192,

		ModelNanoFlash:   "gemini-2.5-flash-image",
		ModelNanoPro:     "gemini-3-pro-image-preview",
		ModelNano2:       "gemini-3.1-flash-image-preview",
		ModelImagen:       "imagen-4.0-generate-001",
		ModelImagenUltra: "imagen-4.0-ultra-generate-001",

		WindowWidth:     950,
		WindowHeight:    700,
		SplitOffsetMain: 0.75,
		SplitOffsetLeft: 0.5,
		SplitOffsetTop:  0.5,
		LogSplitOffset:  0.8,

		ChatMemoryEnabled:   false,
		ChatRememberInitial: false,
		ChatMemorySlots:     3,
		ChatSystemPrompt:    "You are a helpful and professional assistant. Keep your answers concise and accurate.",
		ReturnThought:       false,
	}
}
