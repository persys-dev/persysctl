package models

import (
	"time"
)

type Resources struct {
	CPU    int `json:"cpu"`
	Memory int `json:"memory"`
}

type Workload struct {
	ID            string            `json:"id"`
	Name          string            `json:"name,omitempty"`
	Type          string            `json:"type"`
	Image         string            `json:"image,omitempty"`
	Command       string            `json:"command,omitempty"`
	Compose       string            `json:"compose,omitempty"`
	GitRepo       string            `json:"gitRepo,omitempty"`
	GitBranch     string            `json:"gitBranch,omitempty"`
	GitToken      string            `json:"gitToken,omitempty"`
	LocalPath     string            `json:"localPath,omitempty"`
	EnvVars       map[string]string `json:"envVars,omitempty"`
	Ports         []string          `json:"ports,omitempty"`         // e.g., ["8080:80"]
	Volumes       []string          `json:"volumes,omitempty"`       // e.g., ["/host:/container"]
	Network       string            `json:"network,omitempty"`       // e.g., "bridge"
	RestartPolicy string            `json:"restartPolicy,omitempty"` // e.g., "always"
	Resources     Resources         `json:"resources,omitempty"`
	NodeID        string            `json:"nodeId,omitempty"`
	DesiredState  string            `json:"desiredState,omitempty"`
	Status        string            `json:"status"`
	RevisionID    string            `json:"revisionId,omitempty"`
	RetryAttempts int32             `json:"retryAttempts,omitempty"`
	RetryMax      int32             `json:"retryMaxAttempts,omitempty"`
	RetryNextAt   time.Time         `json:"retryNextAt,omitempty"`
	FailureReason string            `json:"failureReason,omitempty"`
	Message       string            `json:"message,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"createdAt,omitempty"`
	LastUpdated   time.Time         `json:"lastUpdated,omitempty"`
}

type Node struct {
	NodeID        string            `json:"nodeId"`
	IPAddress     string            `json:"ipAddress"`
	Status        string            `json:"status"`
	LastHeartbeat time.Time         `json:"lastHeartbeat"`
	Resources     Resources         `json:"resources"`
	Labels        map[string]string `json:"labels,omitempty"`
}
