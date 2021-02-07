package oss

type objectPageStatus struct {
	delimiter string
	maxKeys   int
	prefix    string
	marker    string
}

func (i *objectPageStatus) ContinuationToken() string {
	return i.marker
}

type storagePageStatus struct {
	marker  string
	maxKeys int
}

func (i *storagePageStatus) ContinuationToken() string {
	return i.marker
}
