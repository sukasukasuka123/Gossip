package NodeManage

func keys[T comparable](m map[T]string) []T {
	ks := make([]T, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
