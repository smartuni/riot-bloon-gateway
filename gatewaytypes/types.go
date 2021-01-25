package gatewaytypes

// MeasuredValue describes a value measured from a balloon
// (temperature, pressure, humidity)
type MeasuredValue struct {
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"`
}

// Balloon contains the data within the balloon
type Balloon struct {
	Temperature []*MeasuredValue `json:"temperature"`
	Humidity    []*MeasuredValue `json:"humidity"`
	Pressure    []*MeasuredValue `json:"pressure"`
	Altitude    float64          `json:"altitude"`
	Longitude   float64          `json:"longitude"`
	Latitude    float64          `json:"latitude"`
	Velocity    float64          `json:"velocity"`
}

// Cluster contains the data within a cluster
type Cluster struct {
	AverageTemp     float64             `json:"avg_temp"`
	AverageHumidity float64             `json:"avg_hum"`
	AveragePressure float64             `json:"avg_pres"`
	Balloons        map[string]*Balloon `json:"balloons"`
}

// ClusterList contains the data for each cluster
type ClusterList struct {
	Clusters map[string]*Cluster `json:"clusters"`
}
