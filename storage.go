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
	"github.com/aos-dev/go-storage/v3/services"
	. "github.com/aos-dev/go-storage/v3/types"
)

func (s *Storage) commitAppend(ctx context.Context, o *Object, opt pairStorageCommitAppend) (err error) {
	return
}

func (s *Storage) completeMultipart(ctx context.Context, o *Object, parts []*Part, opt pairStorageCompleteMultipart) (err error) {
	if o.Mode&ModePart == 0 {
		return services.ObjectModeInvalidError{Expected: ModePart, Actual: o.Mode}
	}

	imur := oss.InitiateMultipartUploadResult{
		Bucket:   s.bucket.BucketName,
		Key:      o.ID,
		UploadID: o.MustGetMultipartID(),
	}

	var uploadParts []oss.UploadPart
	for _, v := range parts {
		uploadParts = append(uploadParts, oss.UploadPart{
			// For user the `PartNumber` is zero-based. But for OSS, the effective `PartNumber` is [1, 10000].
			// Set PartNumber=v.Index+1 here to ensure pass in the effective `PartNumber` for `UploadPart`.
			PartNumber: v.Index + 1,
			ETag:       v.ETag,
		})
	}

	_, err = s.bucket.CompleteMultipartUpload(imur, uploadParts)
	if err != nil {
		return
	}

	o.Mode &= ^ModePart
	o.Mode |= ModeRead
	return
}

func (s *Storage) create(path string, opt pairStorageCreate) (o *Object) {
	// Handle create multipart object separately.
	if opt.HasMultipartID {
		o = s.newObject(true)
		o.Mode = ModePart
		o.SetMultipartID(opt.MultipartID)
	} else {
		o = s.newObject(false)
		o.Mode = ModeRead
	}
	o.ID = s.getAbsPath(path)
	o.Path = path
	return o
}

func (s *Storage) createAppend(ctx context.Context, path string, opt pairStorageCreateAppend) (o *Object, err error) {
	rp := s.getAbsPath(path)

	options := make([]oss.Option, 0)
	options = append(options, oss.ContentLength(0))
	if opt.HasContentType {
		options = append(options, oss.ContentType(opt.ContentType))
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

	offset, err := s.bucket.AppendObject(rp, nil, 0, options...)
	if err != nil {
		return
	}

	o = s.newObject(true)
	o.Mode = ModeRead | ModeAppend
	o.ID = rp
	o.Path = path
	o.SetAppendOffset(offset)
	return o, nil
}

func (s *Storage) createMultipart(ctx context.Context, path string, opt pairStorageCreateMultipart) (o *Object, err error) {
	rp := s.getAbsPath(path)

	options := make([]oss.Option, 0)
	if opt.HasContentType {
		options = append(options, oss.ContentType(opt.ContentType))
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

	output, err := s.bucket.InitiateMultipartUpload(rp, options...)
	if err != nil {
		return
	}

	o = s.newObject(true)
	o.ID = rp
	o.Path = path
	o.Mode |= ModePart
	o.SetMultipartID(output.UploadID)
	// set multipart restriction
	o.SetMultipartNumberMaximum(multipartNumberMaximum)
	o.SetMultipartNumberMaximum(multipartSizeMaximum)
	o.SetMultipartSizeMinimum(multipartSizeMinimum)

	return o, nil
}

func (s *Storage) delete(ctx context.Context, path string, opt pairStorageDelete) (err error) {
	rp := s.getAbsPath(path)

	if opt.HasMultipartID {
		err = s.bucket.AbortMultipartUpload(oss.InitiateMultipartUploadResult{
			Bucket:   s.bucket.BucketName,
			Key:      rp,
			UploadID: opt.MultipartID,
		})
		if err != nil && checkError(err, responseCodeNoSuchUpload) {
			// Omit `NoSuchUpdate` error here
			// ref: [AOS-46](https://github.com/aos-dev/specs/blob/master/rfcs/46-idempotent-delete.md)
			err = nil
		}
		if err != nil {
			return
		}
		return
	}

	// OSS DeleteObject is idempotent, so we don't need to check NoSuchKey error.
	//
	// References
	// - [AOS-46](https://github.com/aos-dev/specs/blob/master/rfcs/46-idempotent-delete.md)
	// - https://help.aliyun.com/document_detail/31982.html
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
	case opt.ListMode.IsPart():
		nextFn = s.nextPartObjectPageByPrefix
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

func (s *Storage) listMultipart(ctx context.Context, o *Object, opt pairStorageListMultipart) (pi *PartIterator, err error) {
	if o.Mode&ModePart == 0 {
		return nil, services.ObjectModeInvalidError{Expected: ModePart, Actual: o.Mode}
	}

	input := &partPageStatus{
		maxParts: 200,
		key:      o.ID,
		uploadId: o.MustGetMultipartID(),
	}

	return NewPartIterator(ctx, s.nextPartPage, input), nil
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

func (s *Storage) nextPartObjectPageByPrefix(ctx context.Context, page *ObjectPage) error {
	input := page.Status.(*objectPageStatus)

	options := make([]oss.Option, 0)
	options = append(options, oss.Delimiter(input.delimiter))
	options = append(options, oss.MaxKeys(input.maxKeys))
	options = append(options, oss.Prefix(input.prefix))
	options = append(options, oss.KeyMarker(input.marker))
	options = append(options, oss.UploadIDMarker(input.partIdMarker))

	output, err := s.bucket.ListMultipartUploads(options...)
	if err != nil {
		return err
	}

	for _, v := range output.Uploads {
		o := s.newObject(true)
		o.ID = v.Key
		o.Path = s.getRelPath(v.Key)
		o.Mode |= ModePart
		o.SetMultipartID(v.UploadID)

		page.Data = append(page.Data, o)
	}

	if output.NextKeyMarker == "" && output.NextUploadIDMarker == "" {
		return IterateDone
	}
	if !output.IsTruncated {
		return IterateDone
	}

	input.marker = output.NextKeyMarker
	input.partIdMarker = output.NextUploadIDMarker
	return nil
}

func (s *Storage) nextPartPage(ctx context.Context, page *PartPage) error {
	input := page.Status.(*partPageStatus)

	output, err := s.bucket.ListUploadedParts(oss.InitiateMultipartUploadResult{
		Bucket:   s.bucket.BucketName,
		Key:      input.key,
		UploadID: input.uploadId,
	})
	if err != nil {
		return err
	}

	for _, v := range output.UploadedParts {
		p := &Part{
			// The returned `PartNumber` is [1, 10000].
			// Set Index=v.PartNumber-1 here to make the `PartNumber` zero-based for user.
			Index: v.PartNumber - 1,
			ETag:  v.ETag,
			Size:  int64(v.Size),
		}

		page.Data = append(page.Data, p)
	}

	if !output.IsTruncated {
		return IterateDone
	}

	partNumberMarker, err := strconv.ParseInt(output.NextPartNumberMarker, 10, 64)
	if err != nil {
		return err
	}

	input.partNumberMarker = partNumberMarker
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

	if opt.HasMultipartID {
		_, err = s.bucket.ListUploadedParts(oss.InitiateMultipartUploadResult{
			Bucket:   s.bucket.BucketName,
			Key:      rp,
			UploadID: opt.MultipartID,
		})
		if err != nil {
			return nil, err
		}

		o = s.newObject(true)
		o.ID = rp
		o.Path = path
		o.Mode |= ModePart
		o.SetMultipartID(opt.MultipartID)
		return o, nil
	}

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
	if !o.Mode.IsAppend() {
		err = fmt.Errorf("object not appendable")
		return
	}

	rp := o.GetID()

	if opt.HasIoCallback {
		r = iowrap.CallbackReader(r, opt.IoCallback)
	}

	offset, ok := o.GetAppendOffset()
	if !ok {
		err = fmt.Errorf("append offset is not set")
		return
	}

	options := make([]oss.Option, 0)
	options = append(options, oss.ContentLength(size))
	if opt.HasContentMd5 {
		options = append(options, oss.ContentMD5(opt.ContentMd5))
	}

	offset, err = s.bucket.AppendObject(rp, r, offset, options...)
	if err != nil {
		return
	}

	o.SetAppendOffset(offset)

	return size, err
}

func (s *Storage) writeMultipart(ctx context.Context, o *Object, r io.Reader, size int64, index int, opt pairStorageWriteMultipart) (n int64, part *Part, err error) {
	if o.Mode&ModePart == 0 {
		return 0, nil, services.ObjectModeInvalidError{Expected: ModePart, Actual: o.Mode}
	}

	imur := oss.InitiateMultipartUploadResult{
		Bucket:   s.bucket.BucketName,
		Key:      o.ID,
		UploadID: o.MustGetMultipartID(),
	}

	options := make([]oss.Option, 0)
	options = append(options, oss.ContentLength(size))
	if opt.HasContentMd5 {
		options = append(options, oss.ContentMD5(opt.ContentMd5))
	}

	// For OSS, the `partNumber` is [1, 10000]. But for user, the `partNumber` is zero-based.
	// Set partNumber=index+1 here to ensure pass in the effective `partNumber` for `UpdatePart`.
	// ref: https://help.aliyun.com/document_detail/31993.html
	output, err := s.bucket.UploadPart(imur, r, size, index+1, options...)
	if err != nil {
		return
	}

	part = &Part{
		// Set part.Index=index instead of part.Index=output.PartNumber to maintain `partNumber` consistency for user.
		Index: index,
		Size:  size,
		ETag:  output.ETag,
	}
	return size, part, nil
}
