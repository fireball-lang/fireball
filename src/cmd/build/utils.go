package build

func runForEach[U, I, R any](uniform U, inputs []I, fun func(U, I) (R, error)) ([]R, error) {
	var items []R
	var err error

	channel := make(chan struct {
		item R
		err  error
	}, 1)

	for _, input := range inputs {
		go func() {
			item, err := fun(uniform, input)

			channel <- struct {
				item R
				err  error
			}{item: item, err: err}
		}()
	}

	count := 0

	for count < len(inputs) {
		result := <-channel

		if result.err != nil {
			err = result.err
		} else {
			items = append(items, result.item)
		}

		count++
	}

	return items, err
}
