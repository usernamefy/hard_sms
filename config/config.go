package config

// App 全局应用配置
var App = struct {
	Port          int    // 监听端口
	DBHost        string // MySQL 主机
	DBPort        int    // MySQL 端口
	DBUser        string // MySQL 用户名
	DBPassword    string // MySQL 密码
	DBName        string // MySQL 库名
	SessionKey    string // 会话加密密钥
	CookieName    string // 会话 Cookie 名称
	SessionMaxAge int    // 会话有效期（秒）
}{
	Port:          8080,
	DBHost:        "127.0.0.1",
	DBPort:        3306,
	DBUser:        "root",
	DBPassword:    "root",
	DBName:        "sms",
	SessionKey:    "sms-session-secret-key-2026",
	CookieName:    "sms_session",
	SessionMaxAge: 86400, // 24 小时
}
