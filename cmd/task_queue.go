package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/SeaCloudAI/seacloud-cli/internal/contracts"
	"github.com/SeaCloudAI/seacloud-cli/internal/queue"
	"github.com/SeaCloudAI/seacloud-cli/internal/taskcache"
)

func getQueueTaskStatus(apiKey, requestID string) (*queue.Task, bool, error) {
	meta, err := taskcache.Load(requestID)
	if errors.Is(err, taskcache.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	if meta.Protocol != "queue" || meta.BodyMode != "raw_json" {
		return nil, false, nil
	}

	contract := contracts.ModelContract{
		ModelID:  meta.ModelID,
		Protocol: meta.Protocol,
		BodyMode: meta.BodyMode,
		Endpoints: contracts.ContractEndpoints{
			Status: contracts.Endpoint{Method: "GET", Path: meta.StatusEndpoint},
			Result: contracts.Endpoint{Method: "GET", Path: meta.ResultEndpoint},
		},
	}
	client := queue.NewClient(apiKey)
	status, err := client.GetStatus(contract, requestID)
	if err != nil {
		return nil, true, err
	}
	saveQueueProviderContext(requestID, status)
	if status.Status == "completed" {
		result, err := client.GetResult(contract, requestID)
		if err == nil {
			saveQueueProviderContext(requestID, result)
		}
		return result, true, err
	}
	return status, true, nil
}

func printQueueTaskStatus(task *queue.Task) error {
	if taskStatusOutput == "url" {
		for _, u := range task.URLs() {
			fmt.Println(u)
		}
		return nil
	}
	if taskStatusOutput == "json" {
		b, _ := json.MarshalIndent(task, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	fmt.Printf("Task:   %s\n", task.ID)
	fmt.Printf("Status: %s\n", task.Status)
	if task.Status == "failed" && task.Error != nil {
		fmt.Printf("Error:  %s\n", task.Error.Message)
	}
	for _, u := range task.URLs() {
		fmt.Printf("URL:    %s\n", u)
	}
	return nil
}
