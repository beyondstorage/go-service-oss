package oss

import "strconv"

type objectPageStatus struct {
	delimiter    string
	maxKeys      int
	prefix       string
	marker       string
	partIdMarker string
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

type partPageStatus struct {
	key              string
	maxParts         int64
	partNumberMarker int64
	uploadId         string
}

func (i *partPageStatus) ContinuationToken() string {
	return strconv.FormatInt(int64(i.partNumberMarker), 10)
}
