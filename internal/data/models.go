package data

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	Name     string `json:"name" bson:"name"`
	Email    string `json:"email" bson:"email"`
	Password string `json:"password" bson:"password"`
}

type Fleet struct {
	UID         string `json:"uid" bson:"uid"`
	Name        string `json:"name" bson:"name"`
	Description string `json:"description" bson:"description"`
	UserID      string `json:"user_id" bson:"user_id"`
}

type Drone struct {
	UID         string        `json:"uid" bson:"uid"`
	Name        string        `json:"name" bson:"name"`
	Description string        `json:"description" bson:"description"`
	FleetID     string        `json:"fleet_id" bson:"fleet_id"`
	Status      ResourceStats `json:"status" bson:"status"`

	DeviceConfig Config `json:"device_config" bson:"device_config"`
}

type DroneCommands struct {
	ID       primitive.ObjectID `bson:"_id"`
	DroneUID string             `json:"drone_uid" bson:"drone_uid"`

	Type    string          `json:"type" bson:"type"`
	Payload json.RawMessage `json:"payload" bson:"payload"`

	Status string `json:"status" bson:"status"` // pending, done, failed

	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`

	ExecutedAt *time.Time `json:"executed_at" bson:"executed_at,omitempty"`
}

type ResourceStats struct {
	CPUStats    CPUInfo       `json:"cpu"`
	MemStat     MemoryInfo    `json:"memory"`
	DiskStats   []DiskInfo    `json:"disks"`
	NetStat     NetStat       `json:"network"`
	NetworkInfo []NetworkInfo `json:"network_info"`
	HostName    string        `json:"host_name"`
	Platform    string        `json:"platform"`
	BootTime    uint64        `json:"boot_time"`

	LastUpdated int64 `json:"last_updated" bson:"last_updated"`
}

type DiskInfo struct {
	Path       string  `json:"path"`
	MountPoint string  `json:"mount_point"`
	TotalGB    float32 `json:"total_gb"`
	UsedGB     float32 `json:"used_gb"`
	UsedPerc   float32 `json:"used_perc"`
}

type MemoryInfo struct {
	TotalGB  float32 `json:"total_gb"`
	UsedGB   float32 `json:"used_gb"`
	UsedPerc float32 `json:"used_perc"`
	CachedGB float32 `json:"cached_gb"`
}

type NetStat struct {
	SentKB uint64 `json:"sent_kb"`
	RecvKB uint64 `json:"recv_kb"`
}

type NetworkInfo struct {
	IP     string `json:"ip"`
	IfName string `json:"ifname"`
	Mac    string `json:"4mac"`
}

type CPUInfo struct {
	TotalUsage    uint8   `json:"total_usage"`
	PhysicalCores int     `json:"physical_cores"`
	LogicalCores  int     `json:"logical_cores"`
	Model         string  `json:"model"`
	Frequency     float64 `json:"frequency"`
}

type TunnelStatus struct {
	ActiveProducers  int                    `json:"active_producers,omitempty"`
	Timestamp        time.Time              `json:"timestamp,omitempty"`
	Topics           map[string]TunnelStats `json:"topics,omitempty"`
	TotalSubscribers int                    `json:"total_subscribers,omitempty"`
	TotalTopics      int                    `json:"total_topics,omitempty"`
}

type TunnelStats struct {
	HasProducer      bool   `json:"has_producer,omitempty"`
	HasWorker        bool   `json:"has_worker,omitempty"`
	MessageQueueSize int    `json:"message_queue_size,omitempty"`
	Name             string `json:"name,omitempty"`
	SubscriberCount  int    `json:"subscriber_count,omitempty"`
}
