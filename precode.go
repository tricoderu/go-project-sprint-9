package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Generator генерирует последовательность чисел 1,2,3 и т.д. и отправляет их в канал ch.
// При этом после записи в канал для каждого числа вызывается функция fn. Она служит для подсчёта количества и суммы сгенерированных чисел.

// так непонятно:
// func Generator(ctx context.Context, chIn chan<- int64, fn func(int64)) {

// так понятнее:
func Generator(ctx context.Context, chIn chan<- int64, fn func(i int64)) {
	// Анонимная функция func(i int64) { ... } определяется непосредственно при вызове Generator
	// 1. Функция Generator
	// ...
	i := int64(1) // начальное значение счетчика
	for {
		select {
		case <-ctx.Done(): // проверяем, не отменён ли контекст ctx (то есть, не закончился ли таймаут). Если контекст отменён, Generator завершает свою работу.
			close(chIn) // Закрываем канал при завершении контекста (время на обработку истекло). Вызов close(chIn) позволяет обрубить все действия, связанные с этим каналом. Это предотвращает ситуацию, когда горутины "зависают", ожидая данные, которые больше не будут поступать.
			// иначе говоря, мы сообщаем всем горутинам, которые ожидают получения данных из этого канала, что канал больше не будет принимать значения и любая попытка отправить в него новое значение будет приводить к панике. Точно так же, любая горутина, которая пытается получить значение из закрытого канала, получит нулевое значение соответствующего типа (например, 0 для int) и специальное булево значение false, означающее, что канал закрыт.
			return // сли бы функция Generator не делала return после закрытия канала, она продолжила бы работать в бесконечном цикле for, ожидая новых значений в канале. Это могло бы привести к блокировке других частей программы, ожидающих завершения Generator. Ну, и мы освобождаем память.
		case chIn <- i: // Число i отправляется в канал ch
			fn(i) // вызываем функцию fn(i), которая служит для подсчёта количества и суммы сгенерированных чисел.
			i++
		}
	}
}

// Worker читает число из канала in и пишет его в канал out.
func Worker(in <-chan int64, out chan<- int64) {
	// 2. Функция Worker
	// ...
	for i := range in { // читаем числа из канала in до тех пор, пока он не будет закрыт.
		out <- i
		time.Sleep(time.Millisecond) // Пауза в 1 миллисекунду
	}
	close(out) // Закрываем out после завершения работы
}

func main() {
	chIn := make(chan int64) // создаем канал in
	// Канал является "небуферизованным" или "синхронным" каналом. В таких каналах операции отправки и получения данных блокируются, пока не найдется соответствующая операция с другой стороны. Как результат, отправка в канал будет блокироваться, пока получатель не получит значение, а получение из канала будет блокироваться, пока отправитель не отправит значение.
	//

	// 3. Создание контекста
	// ...
	// даем программе 5сек для выполнения задачи
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	// cancel - функция, которую можно вызвать для принудительного завершения работы, связанной с этим контекстом.
	// Если эти операции не завершатся самостоятельно, то вызов cancel() принудительно их завершит.
	// Поэтому откладываем
	defer cancel()

	// для проверки будем считать количество и сумму отправленных чисел
	var inputSum int64   // сумма сгенерированных чисел
	var inputCount int64 // количество сгенерированных чисел

	// генерируем числа, считая параллельно их количество и сумму
	// Тут логика обработки данных не требует выделения в отдельную функцию с именем, поэтому мы используем анонимную функцию.
	// Анонимная функция использует переменные из внешнего контекста, создавая замыкание.
	// Так, анонимная функция, передаваемая в Generator, имеет доступ к переменным inputSum и inputCount, объявленным в main(). Это позволяет ей обновлять значения этих переменных при каждом вызове. И горутины совместно используют данные, не прибегая к прямому использованию глобальных переменных.
	// При использовании замыкания, доступ к общим переменным происходит через ссылки, которые хранятся в каждой горутине, использующей это замыкание. Таким образом, каждая горутина имеет свою "копию" ссылки на общие переменные.
	// Но, если мы, например, сохраним ссылку на inputSum или inputCount в глобальную переменную, то в этом случае уже потребуется использование мьютекса для защиты от гонки данных.
	go Generator(ctx, chIn, func(i int64) {
		inputSum += i // увеличиваем значение переменной inputSum на значение, полученное в качестве параметра i
		inputCount++  // увеличиваем значение переменной inputCount на 1. Это позволяет отслеживать, сколько раз была вызвана эта анонимная функция
	})

	const NumOut = 5 // количество обрабатывающих горутин и каналов

	// outs — слайс каналов, куда будут записываться числа из chIn
	outs := make([]chan int64, NumOut) // создаем слайс каналов out размером NumOut=5, заполненный нулевыми значениями (nil). Эти каналы мы будем использовать для параллельной отправки данных из одной горутины chIn в несколько горутин outs.
	for i := 0; i < NumOut; i++ {
		outs[i] = make(chan int64) // создаём каналы
		go Worker(chIn, outs[i])   // запускаем Worker для каждого канала
	}

	// amounts — слайс, в который собирается статистика по горутинам
	amounts := make([]int64, NumOut) // слайс для количества обработанных чисел
	// chOut — канал, в который будут отправляться числа из горутин `outs[i]`

	var wg sync.WaitGroup
	chOut := make(chan int64, NumOut) // результирующий канал
	// 4. Собираем числа из каналов outs
	// ...
	for i := 0; i < NumOut; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for num := range outs[i] { // читаем из каждого канала
				chOut <- num // отправляем в chOut
				amounts[i]++ // считаем количество
			}
		}(i)
	}
	// не сработало
	// wg.Wait() // нужно дождаться завершения всех горутин, подсчитывающих элементы в outs[i], иначе выведет amounts [0 0 0 0 0]

	// Закрываем chOut после завершения всех Worker
	go func() {
		wg.Wait() // ждем завершения работы всех горутин для outs
		close(chOut)
	}()

	var count int64 // количество чисел результирующего канала
	var sum int64   // сумма чисел результирующего канала

	// 5. Читаем числа из результирующего канала
	// ...
	for i := range chOut {
		count++ // count будет хранить общее количество элементов, прочитанных из канала.
		sum += i
	}

	fmt.Println("Количество чисел", inputCount, count)
	fmt.Println("Сумма чисел", inputSum, sum)
	fmt.Println("Разбивка по каналам", amounts)

	// проверка результатов
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
