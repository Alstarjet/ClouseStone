package ctxkeys

// Key is the type used for context value keys in this service.
type Key string

const (
	Email    Key = "email"
	DeviceID Key = "deviceid"
	UserID   Key = "userid"
	Cookie   Key = "cookie"
)
