package breaker

// Breaker adalah interface abstrak untuk circuit breaker.
// Memungkinkan kita menukar implementasi (gobreaker vs FluxGo) tanpa mengubah kode pemanggil.
type Breaker interface {
    // Execute menjalankan sebuah fungsi sinkron dan mengembalikkan response atau error
    Execute(req func() (interface{}, error)) (interface{}, error)
}
