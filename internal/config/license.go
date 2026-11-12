package config

// 已上线服务的公共 API 入口；8088 入口仅放行旧产品路由。
const DeployedLicenseBaseURL = "http://43.142.63.76/api"

// 构建时可注入 HTTPS 入口，未覆盖时使用已上线服务。
var LicenseBaseURL = DeployedLicenseBaseURL
