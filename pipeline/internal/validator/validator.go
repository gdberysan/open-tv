package validator

import (
	"context"
	"sync"
)

// Validator orquesta el proceso concurrente de verificación de streams.
type Validator struct {
	config  Config
	checker *Checker
}

// NewValidator crea un nuevo validador concurrente.
func NewValidator(cfg Config, checker *Checker) *Validator {
	if cfg.MaxWorkers <= 0 {
		cfg = DefaultConfig()
	}
	if checker == nil {
		checker = NewChecker(nil, cfg.Timeout)
	}

	return &Validator{
		config:  cfg,
		checker: checker,
	}
}

// Start consume un canal de URLs entrantes y produce resultados en un canal de salida.
// Utiliza un patrón Worker Pool acotado y Fan-in para consolidar resultados.
func (v *Validator) Start(ctx context.Context, urls <-chan string) <-chan StreamResult {
	results := make(chan StreamResult)

	var wg sync.WaitGroup

	// Lanzar N workers (pool acotado)
	for i := 0; i < v.config.MaxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return // Contexto cancelado, salimos limpiamente
				case url, ok := <-urls:
					if !ok {
						return // Canal de entrada cerrado
					}
					// Realizamos la validación y enviamos el resultado
					res := v.checker.Check(ctx, url)

					// Intentamos enviar al canal de resultados, pero respetamos si ctx se cancela
					select {
					case results <- res:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	// Goroutine adicional que cierra el canal de resultados cuando todos los workers terminan
	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}
