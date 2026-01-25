package secrets

type MongoSecret struct {
	User        string `json:"user"`
	Password    string `json:"password"`
	Database    string `json:"database,omitempty"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	AuthSource  string `json:"auth_source,omitempty"`
	ReplicaSet  string `json:"replica_set,omitempty"`
}

type JWTSecret struct {
	JWTSecret	string	`json:"jwtsecret"`
}

type ServiceKey struct {
	PubSvcKey   string  `json:"pubsvckey"`
	PrivSvcKey  string  `json:"privsvckey"`
}

type Store struct {
    MongoDB    *MongoSecret `json:"mongodb,omitempty"`
    JWTSecret  *JWTSecret   `json:"jwtpass,omitempty"`
    ServiceKey *ServiceKey  `json:"servicekey,omitempty"`
}