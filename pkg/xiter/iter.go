package xiter

import "iter"

type Scan[T any] iter.Seq2[T, error]

// CollectFromScan collects all items and error from an Scan.
func CollectFromScan[T any](it Scan[T]) ([]T, error) {
	var items []T

	var err error

	it(func(item T, e error) bool {
		if e != nil {
			err = e
			return false
		}

		items = append(items, item)

		return true
	})

	return items, err
}
