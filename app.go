package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"os/exec"
	goruntime "runtime"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx    context.Context
	Mu     sync.RWMutex
	Config *Config

	Images    []*ImageInfo
	Tasks     []*TaskInfo
	BatchJobs []*BatchJob

	HTTPClient *http.Client

	NextImageID int
	NextTaskID  int

	BatchMonitorIndex int
	RunningImmediate  int
	RunningBatch      int

	ChatMemory []Content
}

func NewApp() *App {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		cfg = DefaultConfig()
	}

	return &App{
		Config:      cfg,
		NextImageID: 1,
		NextTaskID:  1,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.LoadJobs()

	// Reset task states on startup
	a.Mu.Lock()
	a.RunningImmediate = 0
	a.RunningBatch = 0
	for _, t := range a.Tasks {
		t.RunningCount = 0
		if t.Status == "Running" {
			t.Status = "Failed"
		}
	}
	a.Mu.Unlock()

	// Background Monitoring
	go func() {
		const interval = 2 * time.Minute
		nextCheck := time.Now().Add(interval)
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			a.Mu.RLock()
			jobsLen := len(a.BatchJobs)
			a.Mu.RUnlock()
			if jobsLen == 0 {
				continue
			}

			remaining := time.Until(nextCheck)
			if remaining <= 0 {
				a.Mu.RLock()
				var activeJobs []*BatchJob
				for _, job := range a.BatchJobs {
					s := job.Status
					if s != "SUCCEEDED" && s != "FAILED" && s != "CANCELLED" && s != "EXPIRED" && s != "Success" && s != "Failed" {
						activeJobs = append(activeJobs, job)
					}
				}
				a.Mu.RUnlock()

				if len(activeJobs) > 0 {
					if a.BatchMonitorIndex >= len(activeJobs) {
						a.BatchMonitorIndex = 0
					}
					target := activeJobs[a.BatchMonitorIndex]
					a.BatchMonitorIndex++

					a.Log(fmt.Sprintf("Checking status for job: %s", target.JobID))
					err := a.CheckBatchStatus(target)
					if err != nil {
						a.Log("Status check error: " + err.Error())
					}
					runtime.EventsEmit(a.ctx, "batch_updated")
				}

				nextCheck = time.Now().Add(interval)
				remaining = interval
			}
			runtime.EventsEmit(a.ctx, "batch_timer", int(remaining.Seconds()))
		}
	}()
}

func (a *App) GetConfig() *Config {
	a.Mu.RLock()
	defer a.Mu.RUnlock()
	return a.Config
}

func (a *App) GetDefaultConfig() *Config {
	return DefaultConfig()
}

func (a *App) SaveConfig(cfg *Config) error {
	a.Mu.Lock()
	a.Config = cfg
	a.Mu.Unlock()
	return SaveConfig(cfg)
}

func (a *App) GetImages() []*ImageInfo {
	a.Mu.RLock()
	defer a.Mu.RUnlock()
	return a.Images
}

func (a *App) GetTasks() []*TaskInfo {
	a.Mu.RLock()
	defer a.Mu.RUnlock()
	return a.Tasks
}

func (a *App) Log(msg string) {
	runtime.EventsEmit(a.ctx, "log", msg)

	if a.Config.Debug {
		a.LogToFile(msg)
	}
}

func (a *App) LogToFile(msg string) {
	logPath := "debug.log"
	limit := int64(300 * 1024 * 1024)

	if info, err := os.Stat(logPath); err == nil && info.Size() > limit {
		timestamp := time.Now().Format("2006-01-02-15-04-05")
		os.Rename(logPath, "debug_"+timestamp+".log")
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	f.WriteString("[" + timestamp + "] " + msg + "\n")
}

func (a *App) LoadJobs() {
	f, err := os.Open("jobs.txt")
	if err != nil {
		return
	}
	defer f.Close()

	a.Mu.Lock()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		id := parts[0]
		isFree := false
		if len(parts) > 1 && parts[1] == "free" {
			isFree = true
		}
		a.BatchJobs = append(a.BatchJobs, &BatchJob{
			JobID:       id,
			Status:      "Submitted",
			SubmittedAt: time.Now(),
			Progress:    "0%",
			IsFree:      isFree,
		})
	}
	count := len(a.BatchJobs)
	a.Mu.Unlock()
	a.Log(fmt.Sprintf("Loaded %d batch jobs from jobs.txt", count))
}

func (a *App) ProcessDroppedFiles(paths []string) {
	var imagePaths []string
	var sessionLoaded bool

	for _, p := range paths {
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".json" {
			// Try loading as session
			f, err := os.Open(p)
			if err == nil {
				if err := a.LoadSession(f); err == nil {
					a.Log("Session loaded from dropped file: " + p)
					sessionLoaded = true
				}
				f.Close()
			}
		} else if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			imagePaths = append(imagePaths, p)
		}
	}

	if len(imagePaths) > 0 {
		a.AddImages(imagePaths)
	}

	if sessionLoaded {
		// Sync IDs and refresh
		a.Mu.Lock()
		maxImg := 0
		for _, img := range a.Images {
			var id int
			fmt.Sscanf(img.ID, "%d", &id)
			if id > maxImg { maxImg = id }
		}
		a.NextImageID = maxImg + 1

		maxTask := 0
		for _, t := range a.Tasks {
			if t.ID > maxTask { maxTask = t.ID }
		}
		a.NextTaskID = maxTask + 1
		a.Mu.Unlock()

		runtime.EventsEmit(a.ctx, "images_updated")
		runtime.EventsEmit(a.ctx, "tasks_updated")
	}
}

func (a *App) AddImages(paths []string) {
	a.Mu.Lock()
	defer a.Mu.Unlock()
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}

		w, h := 0, 0
		f, err := os.Open(p)
		if err == nil {
			cfg, _, err := image.DecodeConfig(f)
			if err == nil {
				w = cfg.Width
				h = cfg.Height
			}
			f.Close()
		}

		a.Images = append(a.Images, &ImageInfo{
			ID:       fmt.Sprintf("%d", a.NextImageID),
			FileName: filepath.Base(p),
			FullPath: p,
			SizeMB:   float64(info.Size()) / 1024 / 1024,
			Width:    w,
			Height:   h,
		})
		a.NextImageID++
	}
	runtime.EventsEmit(a.ctx, "images_updated")
}

func (a *App) GetImageBase64(path string) (string, error) {
	if path == "<GENERATE>" || path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func (a *App) SelectAndAddMultipleImages() {
	files, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Images",
		Filters: []runtime.FileFilter{
			{DisplayName: "Images", Pattern: "*.jpg;*.jpeg;*.png"},
		},
	})
	if err != nil {
		a.Log("Error selecting images: " + err.Error())
		return
	}
	if len(files) > 0 {
		a.AddImages(files)
	}
}

func (a *App) CreateNewImage() {
	a.Mu.Lock()
	a.Images = append(a.Images, &ImageInfo{
		ID:       fmt.Sprintf("%d", a.NextImageID),
		FileName: "GENERATE",
		FullPath: "<GENERATE>",
	})
	a.NextImageID++
	a.Mu.Unlock()
	runtime.EventsEmit(a.ctx, "images_updated")
}

func (a *App) getSessionDir() string {
	dir := "session"
	os.MkdirAll(dir, 0755)
	abs, _ := filepath.Abs(dir)
	return abs
}

func (a *App) SaveSessionUI() {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:            "Save Session",
		DefaultFilename:  "session.json",
		DefaultDirectory: a.getSessionDir(),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil || path == "" {
		return
	}

	f, err := os.Create(path)
	if err != nil {
		a.Log("Error creating file: " + err.Error())
		return
	}
	defer f.Close()

	if err := a.SaveSession(f); err != nil {
		a.Log("Error saving session: " + err.Error())
	} else {
		a.Log("Session saved to: " + path)
	}
}

func (a *App) LoadSessionUI() {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Load Session",
		DefaultDirectory: a.getSessionDir(),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil || path == "" {
		return
	}

	f, err := os.Open(path)
	if err != nil {
		a.Log("Error opening file: " + err.Error())
		return
	}
	defer f.Close()

	if err := a.LoadSession(f); err != nil {
		a.Log("Error loading session: " + err.Error())
	} else {
		maxImg := 0
		for _, img := range a.Images {
			var id int
			fmt.Sscanf(img.ID, "%d", &id)
			if id > maxImg {
				maxImg = id
			}
		}
		a.NextImageID = maxImg + 1

		maxTask := 0
		for _, t := range a.Tasks {
			if t.ID > maxTask {
				maxTask = t.ID
			}
		}
		a.NextTaskID = maxTask + 1

		runtime.EventsEmit(a.ctx, "images_updated")
		runtime.EventsEmit(a.ctx, "tasks_updated")
		a.Log("Session loaded from: " + path)
	}
}

func (a *App) AddTask(imgIDs string, agent string, size string, ratio string, prompt string, negPrompt string, paths string, returnThought bool) {
	a.Mu.Lock()
	ids := strings.Split(imgIDs, "+")
	for _, id := range ids {
		id = strings.TrimSpace(id)
		for _, img := range a.Images {
			if img.ID == id {
				img.TaskCount++
				break
			}
		}
	}
	newTask := &TaskInfo{
		ID:             a.NextTaskID,
		ImgIDs:         imgIDs,
		Agent:          agent,
		Size:           size,
		Ratio:          ratio,
		Status:         "Pending",
		Cost:           a.CalculateCost(agent, size, "Immediate"),
		Prompt:         prompt,
		NegativePrompt: negPrompt,
		SourcePath:     paths,
		ReturnThought:  returnThought,
	}
	a.Tasks = append(a.Tasks, newTask)
	a.NextTaskID++
	a.Mu.Unlock()
	runtime.EventsEmit(a.ctx, "tasks_updated")
}

func (a *App) UpdateTask(task *TaskInfo) {
	a.Mu.Lock()
	defer a.Mu.Unlock()

	// Update the task and sync its SourcePath from ImgIDs
	for _, t := range a.Tasks {
		if t.ID == task.ID {
			t.ImgIDs = task.ImgIDs
			t.Agent = task.Agent
			t.Size = task.Size
			t.Ratio = task.Ratio
			t.Prompt = task.Prompt
			t.NegativePrompt = task.NegativePrompt
			t.ReturnThought = task.ReturnThought

			// Resolve SourcePath from ImgIDs
			var paths []string
			ids := strings.Split(t.ImgIDs, "+")
			for _, id := range ids {
				id = strings.TrimSpace(id)
				for _, img := range a.Images {
					if img.ID == id {
						paths = append(paths, img.FullPath)
						break
					}
				}
			}
			t.SourcePath = strings.Join(paths, "|")

			// Recalculate cost
			t.Cost = a.CalculateCost(t.Agent, t.Size, "Immediate")
			break
		}
	}

	// Recalculate TaskCount for all images
	for _, img := range a.Images {
		img.TaskCount = 0
		for _, t := range a.Tasks {
			if a.isIDInMergedID(img.ID, t.ImgIDs) {
				img.TaskCount++
			}
		}
	}

	runtime.EventsEmit(a.ctx, "tasks_updated")
	runtime.EventsEmit(a.ctx, "images_updated")
}

func (a *App) DeleteImage(id string) {
	a.Mu.Lock()
	defer a.Mu.Unlock()
	idx := -1
	for i, img := range a.Images {
		if img.ID == id {
			idx = i
			break
		}
	}
	if idx != -1 {
		a.Images = append(a.Images[:idx], a.Images[idx+1:]...)
		newTasks := []*TaskInfo{}
		for _, t := range a.Tasks {
			if !a.isIDInMergedID(id, t.ImgIDs) {
				newTasks = append(newTasks, t)
			}
		}
		a.Tasks = newTasks
		if len(a.Images) == 0 {
			a.NextImageID = 1
		}
		runtime.EventsEmit(a.ctx, "images_updated")
		runtime.EventsEmit(a.ctx, "tasks_updated")
		a.Log("Image " + id + " deleted.")
	}
}

func (a *App) isIDInMergedID(id, mID string) bool {
	if id == mID {
		return true
	}
	for _, p := range strings.Split(mID, "+") {
		if p == id {
			return true
		}
	}
	return false
}

func (a *App) ClearTasks() {
	a.Mu.Lock()
	a.Tasks = []*TaskInfo{}
	a.NextTaskID = 1
	// Reset TaskCount for all images
	for _, img := range a.Images {
		img.TaskCount = 0
	}
	a.Mu.Unlock()
	a.Log("All tasks cleared.")
	runtime.EventsEmit(a.ctx, "images_updated")
	runtime.EventsEmit(a.ctx, "tasks_updated")
}

func (a *App) ClearImages() {
	a.Mu.Lock()
	a.Images = []*ImageInfo{}
	a.NextImageID = 1
	// Clearing images also clears tasks as they depend on image IDs
	a.Tasks = []*TaskInfo{}
	a.NextTaskID = 1
	a.Mu.Unlock()
	a.Log("All images and tasks cleared.")
	runtime.EventsEmit(a.ctx, "images_updated")
	runtime.EventsEmit(a.ctx, "tasks_updated")
}

func (a *App) DeleteSelectedImages() {
	a.Mu.Lock()
	var remaining []*ImageInfo
	var deletedIDs []string
	for _, img := range a.Images {
		if img.Selected {
			deletedIDs = append(deletedIDs, img.ID)
		} else {
			remaining = append(remaining, img)
		}
	}

	if len(deletedIDs) == 0 {
		a.Mu.Unlock()
		return
	}

	a.Images = remaining
	// Remove tasks associated with deleted images
	var remainingTasks []*TaskInfo
	for _, t := range a.Tasks {
		keep := true
		for _, did := range deletedIDs {
			if a.isIDInMergedID(did, t.ImgIDs) {
				keep = false
				break
			}
		}
		if keep {
			remainingTasks = append(remainingTasks, t)
		}
	}
	a.Tasks = remainingTasks

	if len(a.Images) == 0 {
		a.NextImageID = 1
	}
	if len(a.Tasks) == 0 {
		a.NextTaskID = 1
	}
	a.Mu.Unlock()
	a.Log(fmt.Sprintf("Deleted %d selected images and their associated tasks.", len(deletedIDs)))
	runtime.EventsEmit(a.ctx, "images_updated")
	runtime.EventsEmit(a.ctx, "tasks_updated")
}

func (a *App) DeleteTask(id int) {
	a.Mu.Lock()
	defer a.Mu.Unlock()
	idx := -1
	for i, t := range a.Tasks {
		if t.ID == id {
			idx = i
			break
		}
	}
	if idx != -1 {
		deletedTask := a.Tasks[idx]
		a.Tasks = append(a.Tasks[:idx], a.Tasks[idx+1:]...)
		ids := strings.Split(deletedTask.ImgIDs, "+")
		for _, imgID := range ids {
			imgID = strings.TrimSpace(imgID)
			for _, img := range a.Images {
				if img.ID == imgID {
					if img.TaskCount > 0 {
						img.TaskCount--
					}
					break
				}
			}
		}
		if len(a.Tasks) == 0 {
			a.NextTaskID = 1
		}
		runtime.EventsEmit(a.ctx, "images_updated")
		runtime.EventsEmit(a.ctx, "tasks_updated")
		a.Log("Task deleted.")
	}
}

func (a *App) DuplicateTask(id int) {
	a.Mu.Lock()
	defer a.Mu.Unlock()
	var original *TaskInfo
	for _, t := range a.Tasks {
		if t.ID == id {
			original = t
			break
		}
	}
	if original != nil {
		newTask := *original
		newTask.ID = a.NextTaskID
		a.NextTaskID++
		newTask.Status = "Pending"
		newTask.RunningCount = 0
		newTask.LastSavedPath = ""
		a.Tasks = append(a.Tasks, &newTask)
		runtime.EventsEmit(a.ctx, "tasks_updated")
		a.Log("Task duplicated.")
	}
}

func (a *App) ToggleTaskDisabled(id int) {
	a.Mu.Lock()
	defer a.Mu.Unlock()
	for _, t := range a.Tasks {
		if t.ID == id {
			t.Disabled = !t.Disabled
			break
		}
	}
	runtime.EventsEmit(a.ctx, "tasks_updated")
}

func (a *App) ChangeImageUI(id string) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Change Image",
		Filters: []runtime.FileFilter{{DisplayName: "Images", Pattern: "*.jpg;*.jpeg;*.png"}},
	})
	if err != nil || path == "" {
		return
	}
	a.Mu.Lock()
	defer a.Mu.Unlock()
	for _, img := range a.Images {
		if img.ID == id {
			img.FullPath = path
			img.FileName = filepath.Base(path)
			info, _ := os.Stat(path)
			img.SizeMB = float64(info.Size()) / 1024 / 1024
			for _, t := range a.Tasks {
				if t.ImgIDs == img.ID {
					t.SourcePath = path
				}
			}
			break
		}
	}
	runtime.EventsEmit(a.ctx, "images_updated")
	runtime.EventsEmit(a.ctx, "tasks_updated")
}

func (a *App) RunTasks() {
	a.Log("RunTasks: Starting execution check")
	a.Mu.Lock()
	var tasksToRun []*TaskInfo
	a.Log(fmt.Sprintf("RunTasks: Total tasks in memory: %d", len(a.Tasks)))
	for _, t := range a.Tasks {
		isEligible := !t.Disabled && (t.Status == "Pending" || t.Status == "Failed" || t.Status == "Success" || t.Status == "Running" || t.Status == "Submitted")
		a.Log(fmt.Sprintf(" - Task %d: Agent=%s, Status=%s, Disabled=%v, Eligible=%v", t.ID, t.Agent, t.Status, t.Disabled, isEligible))
		if isEligible {
			tasksToRun = append(tasksToRun, t)
		}
	}
	count := len(tasksToRun)
	a.Mu.Unlock()

	if count == 0 {
		a.Log("RunTasks: No eligible tasks found to run immediately. (Status must be Pending, Failed, Success, or Submitted)")
		runtime.EventsEmit(a.ctx, "run_finished")
		return
	}

	a.Log(fmt.Sprintf("RunTasks: Found %d eligible tasks to execute.", count))
	a.incrementRunningTasks("Immediate")
	go func() {
		defer a.decrementRunningTasks("Immediate")
		if len(tasksToRun) == 1 {
			// If only one task, we just fire off ONE execution.
			// The frontend button logic ensures we don't exceed 2 concurrent runs.
			a.executeTask(tasksToRun[0], "Immediate")
		} else {
			// Multiple tasks: Run them all sequentially.
			for _, task := range tasksToRun {
				a.Mu.RLock()
				isDisabled := task.Disabled
				a.Mu.RUnlock()
				if isDisabled {
					continue
				}
				a.executeTask(task, "Immediate")
			}
		}
	}()
}

func (a *App) incrementRunningTasks(mode string) {
	a.Mu.Lock()
	if mode == "Batch" {
		a.RunningBatch++
		if a.RunningBatch == 1 {
			runtime.EventsEmit(a.ctx, "batch_run_started")
		}
	} else {
		a.RunningImmediate++
		if a.RunningImmediate == 1 {
			runtime.EventsEmit(a.ctx, "run_started")
		}
	}
	imm, bat := a.RunningImmediate, a.RunningBatch
	a.Mu.Unlock()
	a.Log(fmt.Sprintf("Task count increased - Imm: %d, Batch: %d", imm, bat))
}

func (a *App) decrementRunningTasks(mode string) {
	a.Mu.Lock()
	if mode == "Batch" {
		if a.RunningBatch > 0 {
			a.RunningBatch--
		}
		if a.RunningBatch == 0 {
			runtime.EventsEmit(a.ctx, "batch_run_finished")
		}
	} else {
		if a.RunningImmediate > 0 {
			a.RunningImmediate--
		}
		if a.RunningImmediate == 0 {
			runtime.EventsEmit(a.ctx, "run_finished")
		}
	}
	imm, bat := a.RunningImmediate, a.RunningBatch
	a.Mu.Unlock()
	a.Log(fmt.Sprintf("Task count decreased - Imm: %d, Batch: %d", imm, bat))
}

func (a *App) ResetCounters() {
	a.Mu.Lock()
	a.RunningImmediate = 0
	a.RunningBatch = 0
	for _, t := range a.Tasks {
		t.RunningCount = 0
		if t.Status == "Running" {
			t.Status = "Failed"
		}
	}
	a.Mu.Unlock()
	a.Log("All running counters reset manually.")
	runtime.EventsEmit(a.ctx, "tasks_updated")
	runtime.EventsEmit(a.ctx, "run_finished")
	runtime.EventsEmit(a.ctx, "batch_run_finished")
}

func (a *App) executeTask(task *TaskInfo, mode string) {
	a.Log(fmt.Sprintf("executeTask starting for task %d (mode: %s)", task.ID, mode))
	a.Mu.Lock()
	task.Status = "Running"
	task.RunningCount++
	a.Log(fmt.Sprintf("Task %d RunningCount incremented to %d", task.ID, task.RunningCount))
	a.Mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			a.Log(fmt.Sprintf("Recovered from panic in task %d: %v", task.ID, r))
		}
		a.Mu.Lock()
		task.RunningCount--
		if task.RunningCount < 0 {
			task.RunningCount = 0
		}
		a.Mu.Unlock()
		runtime.EventsEmit(a.ctx, "tasks_updated")
	}()

	runtime.EventsEmit(a.ctx, "tasks_updated")
	a.Log(fmt.Sprintf("Running %s...", task.Agent))
	err := a.RunTask(task, mode)

	a.Mu.Lock()
	if err != nil {
		task.Status = "Failed"
		a.Log(fmt.Sprintf("Task %d failed: %v", task.ID, err))
	} else {
		task.Status = "Success"
	}
	a.Mu.Unlock()
}

func (a *App) RunBatch() {
	a.Mu.Lock()
	var tasks []*TaskInfo
	for _, t := range a.Tasks {
		if !t.Disabled && t.Status != "Running" {
			tasks = append(tasks, t)
		}
	}
	a.Mu.Unlock()
	if len(tasks) == 0 {
		a.Log("No tasks to batch.")
		return
	}
	a.incrementRunningTasks("Batch")
	go func() {
		defer a.decrementRunningTasks("Batch")
		err := a.SubmitBatchJob(tasks)
		if err != nil {
			a.Log("Batch failed: " + err.Error())
			a.Mu.Lock()
			for _, t := range tasks { t.Status = "Failed" }
			a.Mu.Unlock()
		} else {
			a.Mu.Lock()
			for _, t := range tasks { t.Status = "Submitted" }
			a.Mu.Unlock()
		}
		runtime.EventsEmit(a.ctx, "tasks_updated")
	}()
}

func (a *App) GetBatchJobs() []*BatchJob {
	a.Mu.RLock()
	defer a.Mu.RUnlock()
	return a.BatchJobs
}

func (a *App) GetRunningTasksCount() int {
	a.Mu.RLock()
	defer a.Mu.RUnlock()
	return a.RunningImmediate + a.RunningBatch
}

func (a *App) ClearChatMemory() {
	a.Mu.Lock()
	a.ChatMemory = nil
	a.Mu.Unlock()
	a.Log("Chat memory cleared.")
}

func (a *App) ClearFinishedJobs() {
	a.Log("Clearing finished batch jobs...")
	a.Mu.Lock()
	var active []*BatchJob
	countBefore := len(a.BatchJobs)
	for _, j := range a.BatchJobs {
		s := strings.ToUpper(j.Status)
		// Terminal states from Gemini API include SUCCEEDED, FAILED, CANCELLED.
		// We also handle normalized Success/Failed and EXPIRED.
		isTerminal := (s == "SUCCEEDED" || s == "FAILED" || s == "CANCELLED" || s == "EXPIRED" || s == "SUCCESS")
		if !isTerminal {
			active = append(active, j)
		} else {
			a.Log(fmt.Sprintf("Removed finished job: %s (Status: %s)", j.JobID, j.Status))
		}
	}
	a.BatchJobs = active
	countAfter := len(a.BatchJobs)
	a.Mu.Unlock()

	a.Log(fmt.Sprintf("Batch cleanup: %d -> %d jobs.", countBefore, countAfter))
	a.CleanupJobsFile()
	runtime.EventsEmit(a.ctx, "batch_updated")
}

func (a *App) OpenImageFolder() {
	out := a.Config.OutputDir
	if out == "" {
		out = "img"
	}
	abs, _ := filepath.Abs(out)

	// Ensure the directory exists
	os.MkdirAll(abs, 0755)

	a.Log("Opening folder: " + abs)

	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", abs)
	case "darwin":
		cmd = exec.Command("open", abs)
	default: // linux, etc
		cmd = exec.Command("xdg-open", abs)
	}

	if err := cmd.Start(); err != nil {
		a.Log("Error opening folder: " + err.Error())
		// Fallback to Wails BrowserOpenURL
		runtime.BrowserOpenURL(a.ctx, abs)
	}
}

func (a *App) GetLastGeneratedImage(taskID int) string {
	lastFile := a.getLastGeneratedImagePath(taskID)
	if lastFile == "" {
		return ""
	}
	b64, _ := a.GetImageBase64(lastFile)
	return b64
}

func (a *App) HasGeneratedImage(taskID int) bool {
	a.Mu.RLock()
	defer a.Mu.RUnlock()
	for _, t := range a.Tasks {
		if t.ID == taskID {
			return t.LastSavedPath != ""
		}
	}
	return false
}

func (a *App) getLastGeneratedImagePath(taskID int) string {
	a.Mu.RLock()
	var lastPath string
	for _, t := range a.Tasks {
		if t.ID == taskID {
			lastPath = t.LastSavedPath
			break
		}
	}
	a.Mu.RUnlock()

	if lastPath != "" {
		if _, err := os.Stat(lastPath); err == nil {
			return lastPath
		}
	}

	// Fallback to searching the directory if path is missing or invalid
	out := a.Config.OutputDir
	if out == "" {
		out = "img"
	}
	files, err := os.ReadDir(out)
	if err != nil {
		return ""
	}
	var lastFile string
	var lastTime time.Time
	// Search for any file in the output dir that might match the task
	// Since we switched to Agent naming, we look for anything with the right timestamp or task ID if we had it
	// But mostly we rely on LastSavedPath now.
	for _, f := range files {
		info, _ := f.Info()
		if info.ModTime().After(lastTime) {
			lastTime = info.ModTime()
			lastFile = filepath.Join(out, f.Name())
		}
	}
	return lastFile
}

func (a *App) GetCost(agent, size, mode string) float64 {
	return a.CalculateCost(agent, size, mode)
}

func (a *App) TestConnection(mode string) {
	go func() {
		runtime.EventsEmit(a.ctx, "test_api_started", mode)
		if err := a.TestAPI(mode); err != nil {
			a.Log("API Test (" + mode + ") Failed: " + err.Error())
			runtime.EventsEmit(a.ctx, "test_api_finished", mode, false, err.Error())
		} else {
			runtime.EventsEmit(a.ctx, "test_api_finished", mode, true, "Success")
		}
	}()
}
