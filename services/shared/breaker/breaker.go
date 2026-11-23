package breaker

import "errors"

// ErrOpen dikembalikan Execute saat breaker menolak request (state open / half-open penuh).
// Pemanggil cukup cek errors.Is(err, breaker.ErrOpen) tanpa tahu implementasi di baliknya.
var ErrOpen = errors.New("circuit breaker is open")

// Breaker adalah interface abstrak untuk circuit breaker.
// Memungkinkan kita menukar implementasi (gobreaker vs FluxGo) tanpa mengubah kode pemanggil.
type Breaker interface {
    // Execute menjalankan sebuah fungsi sinkron dan mengembalikkan response atau error
    Execute(req func() (interface{}, error)) (interface{}, error)
}
