package oss

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"github.com/aos-dev/go-storage/v3/pkg/headers"
	"github.com/aos-dev/go-storage/v3/pkg/iowrap"
	. "github.com/aos-dev/go-storage/v3/types"
)

func (s *Storage) commitAppend(ctx context.Context, o *Object, opt pairStorageCommitAppend) (err error) {
	return
}

func (s *Storage) create(path string, opt pairStorageCreate) (o *Object) {
	o = s.newObject(false)
	o.Mode = ModeRead
	o.ID = s.getAbsPath(path)
	o.Path = path
	return o
}

func (s *Storage) createAppend(ctx context.Context, path string, opt pairStorageCreateAppend) (o *Object, err error) {
	o = s.newObject(true)
	o.Mode = ModeRead | ModeAppend
	o.ID = s.getAbsPath(path)
	o.Path = path
	o.SetAppendOffset(0)
	return o, nil
}

func (s *Storage) delete(ctx context.Context, path string, opt pairStorageDelete) (err error) {
	rp := s.getAbsPath(path)

	err = s.bucket.DeleteObject(rp)
	if err != nil {
		return err
	}
	return nil
}

func (s *Storage) list(ctx context.Context, path string, opt pairStorageList) (oi *ObjectIterator, err error) {
	input := &objectPageStatus{
		maxKeys: 200,
		prefix:  s.getAbsPath(path),
	}

	var nextFn NextObjectFunc

	switch {
	case opt.ListMode.IsDir():
		input.delimiter = "/"
		nextFn = s.nextObjectPageByDir
	case opt.ListMode.IsPrefix():
		nextFn = s.nextObjectPageByPrefix
	default:
		return nil, fmt.Errorf("invalid list mode")
	}

	return NewObjectIterator(ctx, nextFn, input), nil
}

func (s *Storage) metadata(ctx context.Context, opt pairStorageMetadata) (meta *StorageMeta, err error) {
	meta = NewStorageMeta()
	meta.Name = s.bucket.BucketName
	meta.WorkDir = s.workDir
	return
}

func (s *Storage) nextObjectPageByDir(ctx context.Context, page *ObjectPage) error {
	input := page.Status.(*objectPageStatus)

	output, err := s.bucket.ListObjects(
		oss.Marker(input.marker),
		oss.MaxKeys(input.maxKeys),
		oss.Prefix(input.prefix),
		oss.Delimiter(input.delimiter),
	)
	if err != nil {
		return err
	}

	for _, v := range output.CommonPrefixes {
		o := s.newObject(true)
		o.ID = v
		o.Path = s.getRelPath(v)
		o.Mode |= ModeDir

		page.Data = append(page.Data, o)
	}

	for _, v := range output.Objects {
		o, err := s.formatFileObject(v)
		if err != nil {
			return err
		}

		page.Data = append(page.Data, o)
	}

	if !output.IsTruncated {
		return IterateDone
	}

	input.marker = output.NextMarker
	return nil
}

func (s *Storage) nextObjectPageByPrefix(ctx context.Context, page *ObjectPage) error {
	input := page.Status.(*objectPageStatus)

	output, err := s.bucket.ListObjects(
		oss.Marker(input.marker),
		oss.MaxKeys(input.maxKeys),
		oss.Prefix(input.prefix),
	)
	if err != nil {
		return err
	}

	for _, v := range output.Objects {
		o, err := s.formatFileObject(v)
		if err != nil {
			return err
		}

		page.Data = append(page.Data, o)
	}

	if !output.IsTruncated {
		return IterateDone
	}

	input.marker = output.NextMarker
	return nil
}

func (s *Storage) read(ctx context.Context, path string, w io.Writer, opt pairStorageRead) (n int64, err error) {
	rp := s.getAbsPath(path)

	output, err := s.bucket.GetObject(rp)
	if err != nil {
		return 0, err
	}
	defer output.Close()

	rc := output
	if opt.HasIoCallback {
		rc = iowrap.CallbackReadCloser(output, opt.IoCallback)
	}

	return io.Copy(w, rc)
}

func (s *Storage) stat(ctx context.Context, path string, opt pairStorageStat) (o *Object, err error) {
	rp := s.getAbsPath(path)

	output, err := s.bucket.GetObjectMeta(rp)
	if err != nil {
		return nil, err
	}

	o = s.newObject(true)
	o.ID = rp
	o.Path = path
	o.Mode |= ModeRead

	if v := output.Get(headers.ContentLength); v != "" {
		size, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, err
		}
		o.SetContentLength(size)
	}

	if v := output.Get(headers.LastModified); v != "" {
		lastModified, err := time.Parse(time.RFC1123, v)
		if err != nil {
			return nil, err
		}
		o.SetLastModified(lastModified)
	}

	// OSS advise us don't use Etag as Content-MD5.
	//
	// ref: https://help.aliyun.com/document_detail/31965.html
	if v := output.Get(headers.ETag); v != "" {
		o.SetEtag(v)
	}

	if v := output.Get(headers.ContentType); v != "" {
		o.SetContentType(v)
	}

	var sm ObjectMetadata
	if v := output.Get(storageClassHeader); v != "" {
		sm.StorageClass = v
	}
	if v := output.Get(serverSideEncryptionHeader); v != "" {
		sm.ServerSideEncryption = v
	}
	if v := output.Get(serverSideEncryptionKeyIdHeader); v != "" {
		sm.ServerSideEncryptionKeyID = v
	}
	o.SetServiceMetadata(sm)

	return o, nil
}

func (s *Storage) write(ctx context.Context, path string, r io.Reader, size int64, opt pairStorageWrite) (n int64, err error) {
	if opt.HasIoCallback {
		r = iowrap.CallbackReader(r, opt.IoCallback)
	}

	rp := s.getAbsPath(path)

	options := make([]oss.Option, 0)
	options = append(options, oss.ContentLength(size))
	if opt.HasContentMd5 {
		options = append(options, oss.ContentMD5(opt.ContentMd5))
	}
	if opt.HasStorageClass {
		options = append(options, oss.StorageClass(oss.StorageClassType(opt.StorageClass)))
	}
	if opt.HasServerSideEncryption {
		options = append(options, oss.ServerSideEncryption(opt.ServerSideEncryption))
	}
	if opt.HasServerSideDataEncryption {
		options = append(options, oss.ServerSideDataEncryption(opt.ServerSideDataEncryption))
	}
	if opt.HasServerSideEncryptionKeyID {
		options = append(options, oss.ServerSideEncryptionKeyID(opt.ServerSideEncryptionKeyID))
	}

	err = s.bucket.PutObject(rp, r, options...)
	if err != nil {
		return
	}
	return size, nil
}

func (s *Storage) writeAppend(ctx context.Context, o *Object, r io.Reader, size int64, opt pairStorageWriteAppend) (n int64, err error) {
	rp := o.GetID()

	offset, ok := o.GetAppendOffset()
	if !ok {
		err = fmt.Errorf("append offset is not set")
		return
	}

	options := make([]oss.Option, 0)
	options = append(options, oss.ContentLength(size))
	if opt.HasServerSideEncryption {
		options = append(options, oss.ServerSideEncryption(opt.ServerSideEncryption))
	}
	if opt.HasServerSideDataEncryption {
		options = append(options, oss.ServerSideDataEncryption(opt.ServerSideDataEncryption))
	}
	if opt.HasServerSideEncryptionKeyID {
		options = append(options, oss.ServerSideEncryptionKeyID(opt.ServerSideEncryptionKeyID))
	}

	offset, err = s.bucket.AppendObject(rp, r, offset, options...)
	if err != nil {
		return
	}

	o.SetAppendOffset(offset)

	return offset, err
}
