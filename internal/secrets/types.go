package secrets

type MongoSecret struct {
	User       string `json:"user"`
	Password   string `json:"password"`
	Database   string `json:"database,omitempty"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	AuthSource string `json:"auth_source,omitempty"`
	ReplicaSet string `json:"replica_set,omitempty"`
}

type JWTSecret struct {
	JWTSecret string `json:"jwtsecret"`
}

type ServiceKey struct {
	PubSvcKey  string `json:"pubsvckey"`
	PrivSvcKey string `json:"privsvckey"`
}

type AuthToken struct {
	Email       string `json:"email,omitempty"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type,omitempty"`
	AuthURL     string `json:"auth_url,omitempty"`
	LoginPath   string `json:"login_path,omitempty"`
	SavedAt     string `json:"saved_at,omitempty"`
}

type ServerConfig struct {
	BaseURL string `json:"base_url"`
	SavedAt string `json:"saved_at,omitempty"`
}

type Store struct {
	MongoDB           *MongoSecret  `json:"mongodb,omitempty"`
	MongoRootPassword string        `json:"mongo_root_password,omitempty"`
	JWTSecret         *JWTSecret    `json:"jwtpass,omitempty"`
	ServiceKey        *ServiceKey   `json:"servicekey,omitempty"`
	AuthToken         *AuthToken    `json:"auth_token,omitempty"`
	Server            *ServerConfig `json:"server,omitempty"`
}
