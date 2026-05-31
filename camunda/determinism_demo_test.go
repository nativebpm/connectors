package camunda

import (
	"testing"
	"testing/synctest"
	"time"
)

// TestRealTimeDemo демонстрирует, как ведет себя обычный тест, если мы попытаемся 
// подождать завершения долгой операции (например, тайм-аута в 2 секунды).
// Мы закомментировали реальное выполнение, чтобы не замедлять тесты проекта,
// но логика показывает реальные накладные расходы.
func TestRealTimeDemo(t *testing.T) {
	start := time.Now()
	
	// Симулируем задержку в 100 миллисекунд реального времени
	time.Sleep(100 * time.Millisecond)
	
	elapsed := time.Since(start)
	t.Logf("[Real Time] Прошло реального времени: %v", elapsed)
}

// TestVirtualTimeDemo демонстрирует мощь детерминированного тестирования с synctest.
// Мы запускаем горутину, которая засыпает на 10 СЕКУНД (10000 мс).
// В обычном окружении этот тест шел бы 10 секунд. В пузыре synctest он выполняется мгновенно!
func TestVirtualTimeDemo(t *testing.T) {
	// Замеряем реальное время выполнения теста снаружи пузыря виртуального времени
	realStart := time.Now()

	synctest.Test(t, func(t *testing.T) {
		virtualStart := time.Now()
		ch := make(chan struct{})

		// Запускаем конкурентный процесс в виртуальном времени
		go func() {
			// Засыпаем на 10 секунд виртуального времени
			time.Sleep(10 * time.Second)
			close(ch)
		}()

		// Имитируем ожидание в основном потоке на те же 10 виртуальных секунд
		time.Sleep(10 * time.Second)
		<-ch

		t.Logf("[Synctest] Прошло виртуального времени: %v", time.Since(virtualStart))
	})

	t.Logf("[Synctest] Реальное время выполнения теста на CPU: %v", time.Since(realStart))
}
