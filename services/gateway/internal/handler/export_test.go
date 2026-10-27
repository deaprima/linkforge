package handler

import "time"

// SetWSTimings mempercepat keep-alive WebSocket di test; mengembalikan fungsi untuk restore.
func SetWSTimings(ping, pong time.Duration) (restore func()) {
	oldPing, oldPong := wsPingInterval, wsPongWait
	wsPingInterval, wsPongWait = ping, pong
	return func() { wsPingInterval, wsPongWait = oldPing, oldPong }
}
