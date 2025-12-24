package secrets

type MongoSecret struct {
	User        string `json:"user"`
	Password    string `json:"password"`
	Database    string `json:"database,omitempty"`
	Collection  string `json:"collection,omitempty"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	AuthSource  string `json:"auth_source,omitempty"`
	ReplicaSet string `json:"replica_set,omitempty"`
}

type Store struct {
	MongoDB *MongoSecret `json:"mongodb,omitempty"`
}
