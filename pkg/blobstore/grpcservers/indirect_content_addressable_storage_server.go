package grpcservers

import (
	"context"

	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/proto/icas"

	"google.golang.org/grpc/status"
)

type indirectContentAddressableStorageServer struct {
	blobAccess              blobstore.BlobAccess
	maximumMessageSizeBytes int
}

// NewIndirectContentAddressableStorageServer creates a gRPC service for
// serving the contents of an Indirect Content Addressable Storage
// (ICAS). The ICAS is a Buildbarn specific extension for integrating
// external corpora into the CAS.
func NewIndirectContentAddressableStorageServer(blobAccess blobstore.BlobAccess, maximumMessageSizeBytes int) icas.IndirectContentAddressableStorageServer {
	return &indirectContentAddressableStorageServer{
		blobAccess:              blobAccess,
		maximumMessageSizeBytes: maximumMessageSizeBytes,
	}
}

func (s *indirectContentAddressableStorageServer) FindMissingReferences(ctx context.Context, in *remoteexecution.FindMissingBlobsRequest) (*remoteexecution.FindMissingBlobsResponse, error) {
	inDigests := digest.NewSetBuilder()
	for _, partialDigest := range in.BlobDigests {
		digest, err := digest.NewDigestFromPartialDigest(in.InstanceName, partialDigest)
		if err != nil {
			return nil, err
		}
		inDigests.Add(digest)
	}
	outDigests, err := s.blobAccess.FindMissing(ctx, inDigests.Build())
	if err != nil {
		return nil, err
	}
	partialDigests := make([]*remoteexecution.Digest, 0, outDigests.Length())
	for _, outDigest := range outDigests.Items() {
		partialDigests = append(partialDigests, outDigest.GetPartialDigest())
	}
	return &remoteexecution.FindMissingBlobsResponse{
		MissingBlobDigests: partialDigests,
	}, nil
}

func (s *indirectContentAddressableStorageServer) BatchUpdateReferences(ctx context.Context, in *icas.BatchUpdateReferencesRequest) (*remoteexecution.BatchUpdateBlobsResponse, error) {
	responses := make([]*remoteexecution.BatchUpdateBlobsResponse_Response, 0, len(in.Requests))
	for _, request := range in.Requests {
		digest, err := digest.NewDigestFromPartialDigest(in.InstanceName, request.Digest)
		if err == nil {
			err = s.blobAccess.Put(
				ctx,
				digest,
				buffer.NewProtoBufferFromProto(request.Reference, buffer.UserProvided))
		}
		responses = append(responses,
			&remoteexecution.BatchUpdateBlobsResponse_Response{
				Digest: request.Digest,
				Status: status.Convert(err).Proto(),
			})
	}
	return &remoteexecution.BatchUpdateBlobsResponse{
		Responses: responses,
	}, nil
}

func (s *indirectContentAddressableStorageServer) GetReference(ctx context.Context, in *icas.GetReferenceRequest) (*icas.Reference, error) {
	digest, err := digest.NewDigestFromPartialDigest(in.InstanceName, in.Digest)
	if err != nil {
		return nil, err
	}
	actionResult, err := s.blobAccess.Get(ctx, digest).ToProto(
		&icas.Reference{},
		s.maximumMessageSizeBytes)
	if err != nil {
		return nil, err
	}
	return actionResult.(*icas.Reference), nil
}
