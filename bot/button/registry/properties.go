package registry

type Properties struct {
	Flags int
}

func (p *Properties) HasFlag(flag Flag) bool {
	return p.Flags&flag.Int() == flag.Int()
}

func SumFlags(flags ...Flag) (sum int) {
	for _, flag := range flags {
		sum |= flag.Int()
	}

	return sum
}
