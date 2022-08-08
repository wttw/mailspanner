package main

type BailedError string

func (e BailedError) Error() string {
	return string(e)
}
