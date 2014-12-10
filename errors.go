package registry

import (
	"fmt"
	"strings"

	"github.com/docker/docker-registry/digest"
	"github.com/docker/docker-registry/storage"
)

// ErrorCode represents the error type. The errors are serialized via strings
// and the integer format may change and should *never* be exported.
type ErrorCode int

const (
	// ErrorCodeUnknown is a catch-all for errors not defined below.
	ErrorCodeUnknown ErrorCode = iota

	// The following errors can happen during a layer upload.

	// ErrorCodeInvalidDigest is returned when uploading a layer if the
	// provided digest does not match the layer contents.
	ErrorCodeInvalidDigest

	// ErrorCodeInvalidLength is returned when uploading a layer if the provided
	// length does not match the content length.
	ErrorCodeInvalidLength

	// ErrorCodeInvalidName is returned when the name in the manifest does not
	// match the provided name.
	ErrorCodeInvalidName

	// ErrorCodeInvalidTag is returned when the tag in the manifest does not
	// match the provided tag.
	ErrorCodeInvalidTag

	// ErrorCodeUnknownRepository when the repository name is not known.
	ErrorCodeUnknownRepository

	// ErrorCodeUnknownManifest returned when image manifest name and tag is
	// unknown, accompanied by a 404 status.
	ErrorCodeUnknownManifest

	// ErrorCodeInvalidManifest returned when an image manifest is invalid,
	// typically during a PUT operation.
	ErrorCodeInvalidManifest

	// ErrorCodeUnverifiedManifest is returned when the manifest fails signature
	// validation.
	ErrorCodeUnverifiedManifest

	// ErrorCodeUnknownLayer is returned when the manifest references a
	// nonexistent layer.
	ErrorCodeUnknownLayer

	// ErrorCodeUnknownLayerUpload is returned when an upload is accessed.
	ErrorCodeUnknownLayerUpload

	// ErrorCodeUntrustedSignature is returned when the manifest is signed by an
	// untrusted source.
	ErrorCodeUntrustedSignature
)

var errorCodeStrings = map[ErrorCode]string{
	ErrorCodeUnknown:            "UNKNOWN",
	ErrorCodeInvalidDigest:      "INVALID_DIGEST",
	ErrorCodeInvalidLength:      "INVALID_LENGTH",
	ErrorCodeInvalidName:        "INVALID_NAME",
	ErrorCodeInvalidTag:         "INVALID_TAG",
	ErrorCodeUnknownRepository:  "UNKNOWN_REPOSITORY",
	ErrorCodeUnknownManifest:    "UNKNOWN_MANIFEST",
	ErrorCodeInvalidManifest:    "INVALID_MANIFEST",
	ErrorCodeUnverifiedManifest: "UNVERIFIED_MANIFEST",
	ErrorCodeUnknownLayer:       "UNKNOWN_LAYER",
	ErrorCodeUnknownLayerUpload: "UNKNOWN_LAYER_UPLOAD",
	ErrorCodeUntrustedSignature: "UNTRUSTED_SIGNATURE",
}

var errorCodesMessages = map[ErrorCode]string{
	ErrorCodeUnknown:            "unknown error",
	ErrorCodeInvalidDigest:      "provided digest did not match uploaded content",
	ErrorCodeInvalidLength:      "provided length did not match content length",
	ErrorCodeInvalidName:        "manifest name did not match URI",
	ErrorCodeInvalidTag:         "manifest tag did not match URI",
	ErrorCodeUnknownRepository:  "repository not known to registry",
	ErrorCodeUnknownManifest:    "manifest not known",
	ErrorCodeInvalidManifest:    "manifest is invalid",
	ErrorCodeUnverifiedManifest: "manifest failed signature validation",
	ErrorCodeUnknownLayer:       "referenced layer not available",
	ErrorCodeUnknownLayerUpload: "cannot resume unknown layer upload",
	ErrorCodeUntrustedSignature: "manifest signed by untrusted source",
}

var stringToErrorCode map[string]ErrorCode

func init() {
	stringToErrorCode = make(map[string]ErrorCode, len(errorCodeStrings))

	// Build up reverse error code map
	for k, v := range errorCodeStrings {
		stringToErrorCode[v] = k
	}
}

// ParseErrorCode attempts to parse the error code string, returning
// ErrorCodeUnknown if the error is not known.
func ParseErrorCode(s string) ErrorCode {
	ec, ok := stringToErrorCode[s]

	if !ok {
		return ErrorCodeUnknown
	}

	return ec
}

// String returns the canonical identifier for this error code.
func (ec ErrorCode) String() string {
	s, ok := errorCodeStrings[ec]

	if !ok {
		return errorCodeStrings[ErrorCodeUnknown]
	}

	return s
}

// Message returned the human-readable error message for this error code.
func (ec ErrorCode) Message() string {
	m, ok := errorCodesMessages[ec]

	if !ok {
		return errorCodesMessages[ErrorCodeUnknown]
	}

	return m
}

// MarshalText encodes the receiver into UTF-8-encoded text and returns the
// result.
func (ec ErrorCode) MarshalText() (text []byte, err error) {
	return []byte(ec.String()), nil
}

// UnmarshalText decodes the form generated by MarshalText.
func (ec *ErrorCode) UnmarshalText(text []byte) error {
	*ec = stringToErrorCode[string(text)]

	return nil
}

// Error provides a wrapper around ErrorCode with extra Details provided.
type Error struct {
	Code    ErrorCode   `json:"code"`
	Message string      `json:"message,omitempty"`
	Detail  interface{} `json:"detail,omitempty"`
}

// Error returns a human readable representation of the error.
func (e Error) Error() string {
	return fmt.Sprintf("%s: %s",
		strings.ToLower(strings.Replace(e.Code.String(), "_", " ", -1)),
		e.Message)
}

// Errors provides the envelope for multiple errors and a few sugar methods
// for use within the application.
type Errors struct {
	Errors []error `json:"errors,omitempty"`
}

// Push pushes an error on to the error stack, with the optional detail
// argument. It is a programming error (ie panic) to push more than one
// detail at a time.
func (errs *Errors) Push(code ErrorCode, details ...interface{}) {
	if len(details) > 1 {
		panic("please specify zero or one detail items for this error")
	}

	var detail interface{}
	if len(details) > 0 {
		detail = details[0]
	}

	if err, ok := detail.(error); ok {
		detail = err.Error()
	}

	errs.PushErr(Error{
		Code:    code,
		Message: code.Message(),
		Detail:  detail,
	})
}

// PushErr pushes an error interface onto the error stack.
func (errs *Errors) PushErr(err error) {
	switch err.(type) {
	case Error:
		errs.Errors = append(errs.Errors, err)
	default:
		errs.Errors = append(errs.Errors, Error{Message: err.Error()})
	}
}

func (errs *Errors) Error() string {
	switch errs.Len() {
	case 0:
		return "<nil>"
	case 1:
		return errs.Errors[0].Error()
	default:
		msg := "errors:\n"
		for _, err := range errs.Errors {
			msg += err.Error() + "\n"
		}
		return msg
	}
}

// Clear clears the errors.
func (errs *Errors) Clear() {
	errs.Errors = errs.Errors[:0]
}

// Len returns the current number of errors.
func (errs *Errors) Len() int {
	return len(errs.Errors)
}

// DetailUnknownLayer provides detail for unknown layer errors, returned by
// image manifest push for layers that are not yet transferred. This intended
// to only be used on the backend to return detail for this specific error.
type DetailUnknownLayer struct {

	// Unknown should contain the contents of a layer descriptor, which is a
	// single FSLayer currently.
	Unknown storage.FSLayer `json:"unknown"`
}

// RepositoryNotFoundError is returned when making an operation against a
// repository that does not exist in the registry.
type RepositoryNotFoundError struct {
	Name string
}

func (e *RepositoryNotFoundError) Error() string {
	return fmt.Sprintf("No repository found with Name: %s", e.Name)
}

// ImageManifestNotFoundError is returned when making an operation against a
// given image manifest that does not exist in the registry.
type ImageManifestNotFoundError struct {
	Name string
	Tag  string
}

func (e *ImageManifestNotFoundError) Error() string {
	return fmt.Sprintf("No manifest found with Name: %s, Tag: %s",
		e.Name, e.Tag)
}

// BlobNotFoundError is returned when making an operation against a given image
// layer that does not exist in the registry.
type BlobNotFoundError struct {
	Name   string
	Digest digest.Digest
}

func (e *BlobNotFoundError) Error() string {
	return fmt.Sprintf("No blob found with Name: %s, Digest: %s",
		e.Name, e.Digest)
}

// BlobUploadNotFoundError is returned when making a blob upload operation against an
// invalid blob upload location url.
// This may be the result of using a cancelled, completed, or stale upload
// location.
type BlobUploadNotFoundError struct {
	Location string
}

func (e *BlobUploadNotFoundError) Error() string {
	return fmt.Sprintf("No blob upload found at Location: %s", e.Location)
}

// BlobUploadInvalidRangeError is returned when attempting to upload an image
// blob chunk that is out of order.
// This provides the known BlobSize and LastValidRange which can be used to
// resume the upload.
type BlobUploadInvalidRangeError struct {
	Location       string
	LastValidRange int
	BlobSize       int
}

func (e *BlobUploadInvalidRangeError) Error() string {
	return fmt.Sprintf(
		"Invalid range provided for upload at Location: %s. Last Valid Range: %d, Blob Size: %d",
		e.Location, e.LastValidRange, e.BlobSize)
}

// UnexpectedHTTPStatusError is returned when an unexpected HTTP status is
// returned when making a registry api call.
type UnexpectedHTTPStatusError struct {
	Status string
}

func (e *UnexpectedHTTPStatusError) Error() string {
	return fmt.Sprintf("Received unexpected HTTP status: %s", e.Status)
}
