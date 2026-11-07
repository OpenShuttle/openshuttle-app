//go:build !windows && !darwin && !linux

package keepawake

func start(string) (func(), error) {
	return nil, ErrUnsupported
}
