package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

func Generator(ctx context.Context, chIn chan<- int64, fn func(i int64)) {
	i := int64(1) // начальное значение счетчика
	for {
		select {
		case <-ctx.Done():
			close(chIn) // Закрываем канал при завершении контекста
			return
		case chIn <- i: // Число i отправляется в канал ch
			fn(i)
			i++
		}
	}
}

func Worker(in <-chan int64, out chan<- int64, amounts *int64) {
	for i := range in { // читаем числа из канала in
		out <- i
		time.Sleep(time.Millisecond) // Пауза в 1 миллисекунду
		*amounts++                   // Увеличиваем счётчик
	}
	close(out) // Закрываем out после завершения работы
}

func main() {
	chIn := make(chan int64) // создаем канал in

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	var inputSum int64   // сумма сгенерированных чисел
	var inputCount int64 // количество сгенерированных чисел

	go Generator(ctx, chIn, func(i int64) {
		inputSum += i
		inputCount++
	})

	const NumOut = 5                   // количество обрабатывающих горутин и каналов
	outs := make([]chan int64, NumOut) // создаем слайс каналов out
	amounts := make([]int64, NumOut)   // для счетчиков

	for i := 0; i < NumOut; i++ {
		outs[i] = make(chan int64)
		go Worker(chIn, outs[i], &amounts[i])
	}

	// Чтобы собирать данные по всем out в chOut
	chOut := make(chan int64)

	var wg sync.WaitGroup
	wg.Add(NumOut)

	go func() {
		for _, out := range outs {
			for i := range out {
				chOut <- i // отправляем данные в chOut
			}
			wg.Done() // сигнализируем о том, что обработана эта горутина
		}
	}()

	// Закрываем chOut после завершения всех горутин
	go func() {
		wg.Wait()    // ждем завершения всех worker'ов
		close(chOut) // закрываем результирующий канал
	}()

	var count int64 // количество чисел результирующего канала
	var sum int64   // сумма чисел результирующего канала

	// 5. Читаем числа из результирующего канала
	for i := range chOut {
		count++ // count будет хранить общее количество элементов, прочитанных из канала.
		sum += i
	}

	fmt.Println("Количество чисел:", inputCount, count)
	fmt.Println("Сумма чисел:", inputSum, sum)
	fmt.Println("Разбивка по каналам:", amounts)

	// Проверка результатов
	if inputSum != sum {
		log.Fatalf("Ошибка: суммы чисел не равны: %d != %d\n", inputSum, sum)
	}
	if inputCount != count {
		log.Fatalf("Ошибка: количество чисел не равно: %d != %d\n", inputCount, count)
	}

	for _, v := range amounts {
		inputCount -= v
	}
	if inputCount != 0 {
		log.Fatalf("Ошибка: разделение чисел по каналам неверное\n")
	}
}
