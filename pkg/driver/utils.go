package driver

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type VolumeInfo struct {
	ConfigName           string
	Path                 string
	IsV13VolumeIDVersion bool
}

func ParseVolumeID(volumeID string) (VolumeInfo, error) {
	splittedParts := strings.Split(volumeID, ":")
	partsLength := len(splittedParts)
	switch partsLength {
	case 2:
		return VolumeInfo{ConfigName: splittedParts[0], Path: splittedParts[1]}, nil
	case 1:
		return VolumeInfo{Path: splittedParts[0], IsV13VolumeIDVersion: true}, nil
	}
	return VolumeInfo{}, status.Error(codes.InvalidArgument, fmt.Sprintf("Unknown VolumeId format: %s", volumeID))
}
