package gatewaytypes

// import our encoding/json package

// Balloon contains the data within the balloon
type Balloon struct {
	ID          string  `json:"id"`
	Temperature float64 `json:"temperature"`
	Humidity    float64 `json:"humidity"`
	Pressure    float64 `json:"pressure"`
}

// Cluster contains the data within a cluster
type Cluster struct {
	ID              string     `json:"id"`
	AverageTemp     float64    `json:"avg_temp"`
	AverageHumidity float64    `json:"avg_hum"`
	AveragePressure float64    `json:"avg_pres"`
	Balloons        *[]Balloon `json:"balloons"`
}

// ClusterList contains the data for each cluster
type ClusterList struct {
	Clusters *[]Cluster `json:"clusters"`
}
