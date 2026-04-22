package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ImagenRequest struct {
	Instances  []ImagenInstance   `json:"instances"`
	Parameters ImagenParameters   `json:"parameters"`
}

type ImagenInstance struct {
	Prompt string `json:"prompt"`
}

type ImagenParameters struct {
	SampleCount     int    `json:"sampleCount"`
	AspectRatio     string `json:"aspectRatio"`
	SampleImageSize string `json:"sampleImageSize,omitempty"`
}

type GeminiRequest struct {
	Contents          []Content         `json:"contents"`
	SystemInstruction *Content          `json:"systemInstruction,omitempty"`
	SafetySettings    []SafetySetting   `json:"safetySettings"`
	GenerationConfig  *GenerationConfig `json:"generationConfig,omitempty"`
}

type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *InlineData `json:"inlineData,omitempty"`
}

type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type GenerationConfig struct {
	CandidateCount     int          `json:"candidateCount,omitempty"`
	ResponseModalities []string     `json:"responseModalities,omitempty"`
	Temperature        float32      `json:"temperature,omitempty"`
	TopP               float32      `json:"topP,omitempty"`
	TopK               int          `json:"topK,omitempty"`
	MaxOutputTokens    int          `json:"maxOutputTokens,omitempty"`
	ImageConfig        *ImageConfig `json:"imageConfig,omitempty"`
}

type ImageConfig struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
	ImageSize   string `json:"imageSize,omitempty"`
}

func (a *App) GetModelID(agent string) string {
	switch agent {
	case "Nano Flash":
		return a.Config.ModelNanoFlash
	case "Nano Pro":
		return a.Config.ModelNanoPro
	case "Nano 2":
		return a.Config.ModelNano2
	case "Imagen":
		return a.Config.ModelImagen
	case "Imagen Ultra":
		return a.Config.ModelImagenUltra
	}
	return agent // Fallback to raw if not matched
}

func (a *App) CalculateCost(agent, size, mode string) float64 {
	full := agent + " " + size
	base := 0.134 // pro 1K & pro 2k
	nano := true

	if strings.Contains(full, "Imagen 2K") {
		base = 0.04
		nano = false
	}
	if strings.Contains(full, "Ultra 2K") {
		base = 0.06
		nano = false
	}
	if strings.Contains(full, "Pro 4K") {
		base = 0.24
	}
	if strings.Contains(full, "2 1K") {
		base = 0.067
	}
	if strings.Contains(full, "2 2K") {
		base = 0.101
	}
	if strings.Contains(full, "2 4K") {
		base = 0.151
	}
	if strings.Contains(full, "Flash") {
		base = 0.039
	}

	// Apply 50% discount if Batch Mode is selected
	if mode == "Batch" && nano {
		return base * 0.5
	}
	return base
}

func (a *App) getAPIKey(isFree bool) string {
	if isFree {
		return a.Config.APIKeyFree
	}
	return a.Config.APIKeyPaid
}

func (a *App) RunTask(task *TaskInfo, mode string) error {
	// Validate Prompt
	if strings.TrimSpace(task.Prompt) == "" {
		return fmt.Errorf("task prompt is empty")
	}

	// Check file existence
	if task.SourcePath != "" {
		paths := strings.Split(task.SourcePath, "|")
		for _, p := range paths {
			if p == "<GENERATE>" {
				continue
			}
			if _, err := os.Stat(p); os.IsNotExist(err) {
				return fmt.Errorf("file not found: %s", p)
			}
		}
	}

	task.Cost = a.CalculateCost(task.Agent, task.Size, mode)

	modelID := a.GetModelID(task.Agent)
	apiKey := a.getAPIKey(a.Config.IsFreeModeImage)
	modeStr := "Paid"
	if a.Config.IsFreeModeImage {
		modeStr = "Free"
	}

	var url string
	var reqBody []byte
	var err error

	if strings.Contains(task.Agent, "Imagen") {
		url = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:predict?key=%s", modelID, apiKey)
		reqBody, err = a.BuildImagenPayload(task)
	} else {
		url = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", modelID, apiKey)
		reqBody, err = a.BuildPayload(task)
	}

	if err != nil {
		a.Log(fmt.Sprintf("Payload build failed for task %d: %v", task.ID, err))
		return err
	}

	a.Log(fmt.Sprintf("Sending API request for task %d (Model: %s, Billing: %s)", task.ID, modelID, modeStr))
	a.LogToFile(fmt.Sprintf("Task %d Payload: %s", task.ID, string(reqBody)))

	resp, err := a.HTTPClient.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		a.Log(fmt.Sprintf("Network error for task %d: %v", task.ID, err))
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		apiErr := a.HandleError(body, resp.StatusCode)
		a.Log(fmt.Sprintf("API error for task %d (Status %d): %v", task.ID, resp.StatusCode, apiErr))
		return apiErr
	}

	a.Log(fmt.Sprintf("API response received for task %d (Status 200)", task.ID))
	return a.ProcessResponse(body, task)
}

func (a *App) SubmitBatchJob(tasks []*TaskInfo) error {
	if len(tasks) == 0 {
		return nil
	}

	// Validate Prompts and file existence for all tasks
	for _, t := range tasks {
		if strings.TrimSpace(t.Prompt) == "" {
			return fmt.Errorf("task %d prompt is empty", t.ID)
		}
	}

	// Check file existence for all tasks
	for _, t := range tasks {
		if t.SourcePath != "" {
			paths := strings.Split(t.SourcePath, "|")
			for _, p := range paths {
				if p == "<GENERATE>" {
					continue
				}
				if _, err := os.Stat(p); os.IsNotExist(err) {
					return fmt.Errorf("task %d: file not found: %s", t.ID, p)
				}
			}
		}
	}

	modelName := tasks[0].Agent
	modelID := a.GetModelID(modelName)
	isFree := a.Config.IsFreeModeImage
	apiKey := a.getAPIKey(isFree)

	// 1. Create JSONL data
	var buf bytes.Buffer
	for i, t := range tasks {
		// Build the standard payload
		req, err := a.BuildGeminiRequest(t)
		if err != nil {
			a.Log(fmt.Sprintf("Skipping task %d in batch: %v", t.ID, err))
			continue
		}

		// Wrap in Batch format which requires the "model" field inside the request
		type BatchRequest struct {
			Model             string             `json:"model"`
			Contents          []Content          `json:"contents"`
			SystemInstruction *Content           `json:"systemInstruction,omitempty"`
			SafetySettings    []SafetySetting    `json:"safetySettings"`
			GenerationConfig  *GenerationConfig  `json:"generationConfig,omitempty"`
		}

		type BatchReqEntry struct {
			CustomID string       `json:"custom_id"`
			Request  BatchRequest `json:"request"`
		}

		entry := BatchReqEntry{
			CustomID: fmt.Sprintf("task_%d_%d", t.ID, i),
			Request: BatchRequest{
				Model:             "models/" + modelID,
				Contents:          req.Contents,
				SystemInstruction: req.SystemInstruction,
				SafetySettings:    req.SafetySettings,
				GenerationConfig:  req.GenerationConfig,
			},
		}
		line, _ := json.Marshal(entry)
		buf.Write(line)
		buf.WriteString("\n")
	}

	// 2. Upload JSONL to Google Files API
	fileURI, err := a.UploadFile(buf.Bytes(), apiKey)
	if err != nil {
		return fmt.Errorf("upload failed: %v", err)
	}

	// 3. Submit Batch Job
	// Extract resource name (files/...) from URI
	resourceName := fileURI
	if parts := strings.Split(fileURI, "/"); len(parts) > 0 {
		resourceName = "files/" + parts[len(parts)-1]
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:batchGenerateContent?key=%s", modelID, apiKey)

	type BatchSubmitReq struct {
		Batch struct {
			InputConfig struct {
				FileName string `json:"file_name"`
			} `json:"input_config"`
		} `json:"batch"`
	}

	var submitReq BatchSubmitReq
	submitReq.Batch.InputConfig.FileName = resourceName
	reqBody, _ := json.Marshal(submitReq)

	resp, err := a.HTTPClient.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return a.HandleError(body, resp.StatusCode)
	}

	var res struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return err
	}

	a.Mu.Lock()
	a.BatchJobs = append(a.BatchJobs, &BatchJob{
		JobID:       res.Name,
		Status:      "Submitted",
		SubmittedAt: time.Now(),
		Progress:    "0%",
		IsFree:      isFree,
	})
	a.Mu.Unlock()

	a.Log("Batch Job Submitted: " + res.Name)

	// Persist to jobs.txt
	a.CleanupJobsFile()

	return nil
}

func (a *App) UploadFile(data []byte, apiKey string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/upload/v1beta/files?key=%s", apiKey)

	boundary := "NanoGoBoundary" + fmt.Sprint(time.Now().Unix())
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.SetBoundary(boundary)

	// Part 1: Metadata
	metadata := fmt.Sprintf(`{"file": {"display_name": "b%d"}}`, time.Now().Unix())
	h := make(textproto.MIMEHeader)
	h.Set("Content-Type", "application/json; charset=UTF-8")
	p, _ := writer.CreatePart(h)
	p.Write([]byte(metadata))

	// Part 2: File Content
	h = make(textproto.MIMEHeader)
	h.Set("Content-Type", "application/json")
	p, _ = writer.CreatePart(h)
	p.Write(data)

	writer.Close()

	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("X-Goog-Upload-Protocol", "multipart")
	req.Header.Set("Content-Type", "multipart/related; boundary="+boundary)

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	resBody, _ := io.ReadAll(resp.Body)

	var res struct {
		File struct {
			URI string `json:"uri"`
		} `json:"file"`
	}
	json.Unmarshal(resBody, &res)

	if res.File.URI == "" {
		return "", fmt.Errorf("no URI in response: %s", string(resBody))
	}
	return res.File.URI, nil
}

func (a *App) BuildImagenPayload(task *TaskInfo) ([]byte, error) {
	req := ImagenRequest{
		Instances: []ImagenInstance{{Prompt: task.Prompt}},
		Parameters: ImagenParameters{
			SampleCount: 1,
			AspectRatio: task.Ratio,
		},
	}
	if task.Size != "1K" {
		req.Parameters.SampleImageSize = task.Size
	}
	return json.Marshal(req)
}

func getMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".heic":
		return "image/heic"
	case ".heif":
		return "image/heif"
	default:
		return "image/jpeg"
	}
}

func (a *App) BuildGeminiRequest(task *TaskInfo) (*GeminiRequest, error) {
	prompt := task.Prompt
	if task.ReturnThought {
		prompt += "\n\nCRITICAL: Before generating the image, you MUST provide a detailed written analysis and description of your creative process and how you are interpreting the user's directive. This text will be saved as your 'creative thought'."
	}
	fullPrompt := fmt.Sprintf("USER DIRECTIVE: %s. Aspect Ratio: %s. Avoid: %s", prompt, task.Ratio, task.NegativePrompt)
	parts := []Part{{Text: fullPrompt}}
	encourage := a.Config.EncourageGen

	modalities := []string{"IMAGE"}
	if task.ReturnThought {
		modalities = append(modalities, "TEXT")
	}

	if task.SourcePath != "" {
		hasRealImages := false
		paths := strings.Split(task.SourcePath, "|")
		for _, p := range paths {
			if p == "<GENERATE>" {
				continue
			}
			hasRealImages = true
			data, err := os.ReadFile(p)
			if err != nil {
				return nil, fmt.Errorf("could not read image %s: %v", p, err)
			}
			b64 := base64.StdEncoding.EncodeToString(data)
			parts = append(parts, Part{
				InlineData: &InlineData{
					MimeType: getMimeType(p),
					Data:     b64,
				},
			})
		}
		if hasRealImages {
			encourage = a.Config.EncourageEdt
		}
	}

	req := &GeminiRequest{
		Contents: []Content{{Parts: parts}},
		SystemInstruction: &Content{Parts: []Part{{
			Text: encourage,
		}}},
		SafetySettings: a.Config.SafetySettings,
		GenerationConfig: &GenerationConfig{
			CandidateCount:     1,
			ResponseModalities: modalities,
			Temperature:        a.Config.Temperature,
			TopP:               a.Config.TopP,
			TopK:               a.Config.TopK,
			MaxOutputTokens:    a.Config.MaxOutputTokens,
			ImageConfig: &ImageConfig{
				AspectRatio: task.Ratio,
				ImageSize:   task.Size,
			},
		},
	}

	return req, nil
}

func (a *App) BuildPayload(task *TaskInfo) ([]byte, error) {
	req, err := a.BuildGeminiRequest(task)
	if err != nil {
		return nil, err
	}
	return json.Marshal(req)
}

func (a *App) HandleError(body []byte, status int) error {
	// Ported InspectApiResponse logic
	bodyStr := string(body)
	if strings.Contains(bodyStr, "<html") {
		re := regexp.MustCompile(`(?i)<h1>(.*?)</h1>`)
		match := re.FindStringSubmatch(bodyStr)
		if len(match) > 1 {
			return fmt.Errorf("HTML Error: %s", match[1])
		}
		return fmt.Errorf("HTTP Error %d (HTML response)", status)
	}

	// Handle structured JSON error
	type ErrorDetail struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	}
	type ErrorResp struct {
		Error ErrorDetail `json:"error"`
	}

	var errResp ErrorResp
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Errorf("API Error %d (%s): %s", errResp.Error.Code, errResp.Error.Status, errResp.Error.Message)
	}

	// Handle array-wrapped error (user example)
	var errArray []ErrorResp
	if err := json.Unmarshal(body, &errArray); err == nil && len(errArray) > 0 && errArray[0].Error.Message != "" {
		return fmt.Errorf("API Error %d (%s): %s", errArray[0].Error.Code, errArray[0].Error.Status, errArray[0].Error.Message)
	}

	return fmt.Errorf("HTTP Error %d: %s", status, bodyStr)
}

func (a *App) TestAPI(mode string) error {
	apiKey := a.Config.APIKeyPaid
	if mode == "free" {
		apiKey = a.Config.APIKeyFree
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", apiKey)
	resp, err := a.HTTPClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return a.HandleError(body, resp.StatusCode)
	}

	var modelsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return err
	}

	a.Log(fmt.Sprintf("Connection successful. Found %d models.", len(modelsResp.Models)))

	// Log first 10 models to UI
	limit := 10
	if len(modelsResp.Models) < limit {
		limit = len(modelsResp.Models)
	}
	for i := 0; i < limit; i++ {
		a.Log(" - " + modelsResp.Models[i].Name)
	}

	// Log ALL models to debug.log file unconditionally
	a.LogToFile("Full Model List:")
	for _, m := range modelsResp.Models {
		a.LogToFile(" - " + m.Name)
	}

	if len(modelsResp.Models) > limit {
		a.Log(fmt.Sprintf(" ... and %d more. Full list in debug.log", len(modelsResp.Models)-limit))
	}
	return nil
}

func (a *App) findJSONString(data interface{}, targetKey string) string {
	switch v := data.(type) {
	case map[string]interface{}:
		for k, val := range v {
			if k == targetKey {
				if str, ok := val.(string); ok {
					return str
				}
			}
			if found := a.findJSONString(val, targetKey); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, item := range v {
			if found := a.findJSONString(item, targetKey); found != "" {
				return found
			}
		}
	}
	return ""
}

func (a *App) CheckBatchStatus(job *BatchJob) error {
	apiKey := a.getAPIKey(job.IsFree)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s?key=%s", job.JobID, apiKey)
	resp, err := a.HTTPClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return a.HandleError(body, resp.StatusCode)
	}

	var res interface{}
	if err := json.Unmarshal(body, &res); err != nil {
		return err
	}

	// Log raw response to debug file for troubleshooting
	a.LogToFile(fmt.Sprintf("Raw Status Response for %s: %s", job.JobID, string(body)))

	// Robustly find 'state' and normalization
	state := a.findJSONString(res, "state")
	if state == "" {
		if job.Status == "" {
			state = "UNKNOWN"
		} else {
			state = job.Status // Keep current if empty from API
		}
	}
	if strings.HasPrefix(state, "BATCH_STATE_") {
		state = state[len("BATCH_STATE_"):]
	}
	job.Status = state
	a.Log(fmt.Sprintf("Batch %s status: %s", job.JobID, state))

	if state == "SUCCEEDED" {
		respFile := a.findJSONString(res, "responseFile")
		if respFile == "" {
			respFile = a.findJSONString(res, "responsesFile")
		}

		if respFile != "" {
			a.Log("Batch " + job.JobID + " succeeded. Downloading results...")
			return a.DownloadBatchResults(respFile, job.IsFree)
		}
	} else if state == "FAILED" || state == "CANCELLED" || state == "EXPIRED" {
		a.CleanupJobsFile()
	}
	return nil
}

func (a *App) CleanupJobsFile() {
	a.Mu.RLock()
	var lines []string
	for _, job := range a.BatchJobs {
		s := strings.ToUpper(job.Status)
		if s != "SUCCEEDED" && s != "FAILED" && s != "CANCELLED" && s != "EXPIRED" && s != "SUCCESS" {
			mode := "paid"
			if job.IsFree {
				mode = "free"
			}
			lines = append(lines, fmt.Sprintf("%s|%s", job.JobID, mode))
		}
	}
	a.Mu.RUnlock()

	if len(lines) == 0 {
		os.Remove("jobs.txt")
		return
	}

	f, err := os.Create("jobs.txt")
	if err != nil {
		return
	}
	defer f.Close()

	for _, line := range lines {
		f.WriteString(line + "\n")
	}
}

func (a *App) DownloadBatchResults(fileID string, isFree bool) error {
	apiKey := a.getAPIKey(isFree)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s:download?alt=media&key=%s", fileID, apiKey)
	resp, err := a.HTTPClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// JSONL response with multiple lines
	reader := io.Reader(resp.Body)
	decoder := json.NewDecoder(reader)

	for {
		var result struct {
			CustomID string          `json:"custom_id"`
			Response json.RawMessage `json:"response"`
			Error    json.RawMessage `json:"error"`
		}
		if err := decoder.Decode(&result); err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if len(result.Error) > 0 {
			a.Log("Task " + result.CustomID + " failed in batch: " + string(result.Error))
			var taskID int
			fmt.Sscanf(result.CustomID, "task_%d_", &taskID)
			a.Mu.Lock()
			for _, t := range a.Tasks {
				if t.ID == taskID {
					t.Status = "Failed"
					break
				}
			}
			a.Mu.Unlock()
			continue
		}

		// Port image extraction logic from ProcessResponse for result.Response
		a.ProcessBatchItem(result.Response, result.CustomID)
	}

	a.CleanupJobsFile()
	runtime.EventsEmit(a.ctx, "tasks_updated")
	return nil
}

func (a *App) ProcessBatchItem(respBody []byte, customID string) {
	// customID is task_{taskID}_{index}
	var taskID int
	fmt.Sscanf(customID, "task_%d_", &taskID)

	// Nested parsing for image and text data
	var resp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text       string `json:"text,omitempty"`
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBody, &resp); err == nil && len(resp.Candidates) > 0 {
		cand := resp.Candidates[0]
		a.Mu.Lock()
		var targetTask *TaskInfo
		for _, t := range a.Tasks {
			if t.ID == taskID {
				targetTask = t
				break
			}
		}
		a.Mu.Unlock()

		if targetTask == nil {
			return
		}

		var imageData []byte
		var mimeType string
		var aiText string

		for _, part := range cand.Content.Parts {
			if part.InlineData != nil && part.InlineData.Data != "" {
				imageData, _ = base64.StdEncoding.DecodeString(part.InlineData.Data)
				mimeType = part.InlineData.MimeType
			} else if part.Text != "" {
				aiText += part.Text
			}
		}

		if imageData != nil || aiText != "" {
			agentPrefix := strings.ReplaceAll(targetTask.Agent, " ", "_")
			randomHex := fmt.Sprintf("%02x", time.Now().UnixNano()%256)
			dateStr := time.Now().Format("2006-01-02-150405")
			os.MkdirAll(a.Config.OutputDir, 0755)

			var primaryPath string

			if imageData != nil {
				ext := "jpg"
				if strings.Contains(strings.ToLower(mimeType), "png") {
					ext = "png"
				}

				fileName := fmt.Sprintf("%s_%s_%s.%s", agentPrefix, dateStr, randomHex, ext)
				primaryPath = filepath.Join(a.Config.OutputDir, fileName)

				if err := os.WriteFile(primaryPath, imageData, 0644); err != nil {
					a.Log(fmt.Sprintf("Error saving image for task %d: %v", targetTask.ID, err))
				} else {
					a.Log(fmt.Sprintf("Saved task %d image: %s", targetTask.ID, primaryPath))
				}

				if targetTask.ReturnThought {
					if aiText != "" {
						txtPath := primaryPath[:len(primaryPath)-len(ext)-1] + ".txt"
						if err := os.WriteFile(txtPath, []byte(aiText), 0644); err != nil {
							a.Log("Error saving thoughts: " + err.Error())
						} else {
							a.Log("Saved AI thoughts: " + txtPath)
						}
					} else {
						a.Log(fmt.Sprintf("Warning: Thought process requested for task %d but no text was returned.", targetTask.ID))
					}
				}
			} else {
				// Text only response
				fileName := fmt.Sprintf("%s_%s_%s.txt", agentPrefix, dateStr, randomHex)
				primaryPath = filepath.Join(a.Config.OutputDir, fileName)

				if err := os.WriteFile(primaryPath, []byte(aiText), 0644); err != nil {
					a.Log(fmt.Sprintf("Error saving text response for task %d: %v", targetTask.ID, err))
				} else {
					a.Log(fmt.Sprintf("Saved task %d AI response (text only): %s", targetTask.ID, primaryPath))
				}
			}

			a.Mu.Lock()
			targetTask.LastSavedPath = primaryPath
			targetTask.Status = "Success"
			a.Mu.Unlock()
		}
	}
}

func (a *App) CalculateChatCost(model, message string) float64 {
	totalChars := len(message)

	a.Mu.RLock()
	if a.Config.ChatMemoryEnabled {
		// Calculate what will actually be sent
		slots := a.Config.ChatMemorySlots
		if slots < 1 { slots = 1 }

		var history []Content
		if a.Config.ChatRememberInitial && len(a.ChatMemory) >= 2 {
			initial := a.ChatMemory[:2]
			rolling := a.ChatMemory[2:]
			limit := (slots - 2) * 2
			if limit < 0 { limit = 0 }
			if len(rolling) > limit {
				rolling = rolling[len(rolling)-limit:]
			}
			history = append(initial, rolling...)
		} else {
			limit := (slots - 1) * 2
			if limit < 0 { limit = 0 }
			history = a.ChatMemory
			if len(history) > limit {
				history = history[len(history)-limit:]
			}
		}

		for _, content := range history {
			for _, part := range content.Parts {
				totalChars += len(part.Text)
			}
		}
	}
	a.Mu.RUnlock()

	tokens := totalChars / 4
	if tokens < 1 {
		tokens = 1
	}
	pricePer1K := 0.000125 // conservative
	if strings.Contains(model, "pro") {
		pricePer1K = 0.00125
	}
	return (float64(tokens) / 1000.0) * pricePer1K
}

func (a *App) SendChatMessage(model, message string) (string, error) {
	apiKey := a.getAPIKey(a.Config.IsFreeModeChat)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)
	a.Log(fmt.Sprintf("Sending Chat request to model: %s", model))

	a.Mu.Lock()
	if !a.Config.ChatMemoryEnabled {
		a.ChatMemory = nil
	}

	newMsg := Content{Role: "user", Parts: []Part{{Text: message}}}

	var contents []Content
	if a.Config.ChatMemoryEnabled {
		slots := a.Config.ChatMemorySlots
		if slots < 1 { slots = 1 }

		if a.Config.ChatRememberInitial && len(a.ChatMemory) >= 2 {
			initial := a.ChatMemory[:2]
			rolling := a.ChatMemory[2:]
			limit := (slots - 2) * 2
			if limit < 0 { limit = 0 }
			if len(rolling) > limit {
				rolling = rolling[len(rolling)-limit:]
			}
			contents = append(initial, rolling...)
		} else {
			limit := (slots - 1) * 2
			if limit < 0 { limit = 0 }
			history := a.ChatMemory
			if len(history) > limit {
				history = history[len(history)-limit:]
			}
			contents = history
		}
		contents = append(contents, newMsg)
	} else {
		contents = []Content{newMsg}
	}
	a.Mu.Unlock()

	req := GeminiRequest{
		Contents: contents,
		SystemInstruction: &Content{Parts: []Part{{Text: a.Config.ChatSystemPrompt}}},
		GenerationConfig: &GenerationConfig{
			CandidateCount:  1,
			Temperature:     a.Config.Temperature,
			TopP:            a.Config.TopP,
			TopK:            a.Config.TopK,
			MaxOutputTokens: a.Config.MaxOutputTokens,
		},
		SafetySettings: a.Config.SafetySettings,
	}

	reqBody, _ := json.Marshal(req)
	resp, err := a.HTTPClient.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", a.HandleError(body, resp.StatusCode)
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", err
	}

	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		reply := geminiResp.Candidates[0].Content.Parts[0].Text

		a.Mu.Lock()
		if a.Config.ChatMemoryEnabled {
			// Append both user message and AI reply to memory in pairs
			a.ChatMemory = append(a.ChatMemory, Content{Role: "user", Parts: []Part{{Text: message}}})
			a.ChatMemory = append(a.ChatMemory, Content{Role: "model", Parts: []Part{{Text: reply}}})
		}
		a.Mu.Unlock()

		return reply, nil
	}

	return "", fmt.Errorf("no response text from AI")
}

func (a *App) ProcessResponse(body []byte, task *TaskInfo) error {
	a.LogToFile(fmt.Sprintf("DEBUG: Processing response for task %d. Body length: %d", task.ID, len(body)))
	// 1. Try Gemini Candidates format
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text       string `json:"text,omitempty"`
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &geminiResp); err == nil && len(geminiResp.Candidates) > 0 {
		cand := geminiResp.Candidates[0]
		if cand.FinishReason != "" && cand.FinishReason != "STOP" && cand.FinishReason != "SUCCESS" {
			return fmt.Errorf("finish reason: %s", cand.FinishReason)
		}
		var imageData string
		var mimeType string
		var aiText string

		for _, part := range cand.Content.Parts {
			if part.InlineData != nil && part.InlineData.Data != "" {
				imageData = part.InlineData.Data
				mimeType = part.InlineData.MimeType
			} else if part.Text != "" {
				aiText += part.Text
			}
		}

		if imageData != "" || aiText != "" {
			return a.SaveBase64Image(imageData, mimeType, task, aiText)
		}
	}

	// 2. Try Imagen predictions format
	var imagenResp struct {
		Predictions []struct {
			MimeType           string `json:"mimeType"`
			BytesBase64Encoded string `json:"bytesBase64Encoded"`
		} `json:"predictions"`
	}
	if err := json.Unmarshal(body, &imagenResp); err == nil && len(imagenResp.Predictions) > 0 {
		pred := imagenResp.Predictions[0]
		return a.SaveBase64Image(pred.BytesBase64Encoded, pred.MimeType, task, "")
	}

	a.LogToFile(fmt.Sprintf("DEBUG: No image found in response for task %d. Body: %s", task.ID, string(body)))
	return fmt.Errorf("no image data in response (Check debug.log for body)")
}

func (a *App) SaveBase64Image(b64, mime string, task *TaskInfo, aiText string) error {
	agentPrefix := strings.ReplaceAll(task.Agent, " ", "_")
	randomHex := fmt.Sprintf("%02x", time.Now().UnixNano()%256)
	dateStr := time.Now().Format("2006-01-02-150405")
	os.MkdirAll(a.Config.OutputDir, 0755)

	var primaryPath string

	if b64 != "" {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return err
		}

		ext := "jpg"
		if strings.Contains(strings.ToLower(mime), "png") {
			ext = "png"
		}

		fileName := fmt.Sprintf("%s_%s_%s.%s", agentPrefix, dateStr, randomHex, ext)
		primaryPath = filepath.Join(a.Config.OutputDir, fileName)

		if err := os.WriteFile(primaryPath, data, 0644); err != nil {
			return err
		}
		a.Log("Saved image to: " + primaryPath)

		if task.ReturnThought {
			if aiText != "" {
				txtPath := primaryPath[:len(primaryPath)-len(ext)-1] + ".txt"
				if err := os.WriteFile(txtPath, []byte(aiText), 0644); err != nil {
					a.Log("Error saving thoughts: " + err.Error())
				} else {
					a.Log("Saved AI thoughts: " + txtPath)
				}
			} else {
				a.Log(fmt.Sprintf("Warning: Thought process requested for task %d but no text was returned.", task.ID))
			}
		}
	} else if aiText != "" {
		// Text only response
		fileName := fmt.Sprintf("%s_%s_%s.txt", agentPrefix, dateStr, randomHex)
		primaryPath = filepath.Join(a.Config.OutputDir, fileName)

		if err := os.WriteFile(primaryPath, []byte(aiText), 0644); err != nil {
			return err
		}
		a.Log("Saved AI response (text only) to: " + primaryPath)
	} else {
		return fmt.Errorf("empty response (no image or text)")
	}

	a.Mu.Lock()
	task.LastSavedPath = primaryPath
	a.Mu.Unlock()
	return nil
}
