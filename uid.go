package main

var cUID chan uint

func init() {
	cUID = make(chan uint, 16)
	go func() {
		var i uint
		for {
			cUID <- i
			i++
		}
	}()
}

func getUID() uint {
	return <-cUID
}
