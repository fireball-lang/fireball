package build

func runForEach[I, R any](inputs []I, fun func(I) (R, error)) ([]R, error) {
	var items []R
	var err error

	channel := make(chan struct {
		item R
		err  error
	}, 1)

	for _, input := range inputs {
		go func() {
			item, err := fun(input)

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
