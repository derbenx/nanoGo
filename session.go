package main

import (
	"encoding/json"
	"io"
)

type SessionData struct {
	Images []*ImageInfo `json:"images"`
	Tasks  []*TaskInfo  `json:"tasks"`
}

func (a *App) SaveSession(w io.Writer) error {
	data := SessionData{
		Images: a.Images,
		Tasks:  a.Tasks,
	}
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	_, err = w.Write(bytes)
	return err
}

func (a *App) LoadSession(r io.Reader) error {
	bytes, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	var data SessionData
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		return err
	}

	a.Images = data.Images
	a.Tasks = data.Tasks
	for _, t := range a.Tasks {
		t.RunningCount = 0
		if t.Status == "Running" {
			t.Status = "Failed"
		}
	}
	return nil
}
