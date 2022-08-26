package redpacket

type RedPacketDataError struct {
	message string
}

func newRedPacketDataError(message string) error {
	return &RedPacketDataError{message: message}
}

func (e *RedPacketDataError) Error() string {
	return e.message
}
