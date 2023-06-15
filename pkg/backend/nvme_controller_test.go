// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2023 Intel Corporation

// Package backend implememnts the BackEnd APIs (network facing) of the storage Server
package backend

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

var (
	controllerID   = "opi-nvme8"
	controllerName = server.ResourceIDToVolumeName(controllerID)
	controller     = pb.NVMfRemoteController{
		Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
		Adrfam:  pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
		Traddr:  "127.0.0.1",
		Trsvcid: 4444,
		Subnqn:  "nqn.2016-06.io.spdk:cnode1",
		Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
	}
)

func TestBackEnd_CreateNVMfRemoteController(t *testing.T) {
	tests := map[string]struct {
		id      string
		in      *pb.NVMfRemoteController
		out     *pb.NVMfRemoteController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		exist   bool
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&controller,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
			false,
		},
		"valid request with invalid marshal SPDK response": {
			controllerID,
			&controller,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_attach_controller: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevNvmeAttachControllerResult"),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			controllerID,
			&controller,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_attach_controller: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			controllerID,
			&controller,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_attach_controller: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			controllerID,
			&controller,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_attach_controller: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			controllerID,
			&controller,
			&controller,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":["my_remote_nvmf_bdev"]}`},
			codes.OK,
			"",
			true,
			false,
		},
		"already exists": {
			controllerID,
			&controller,
			&controller,
			[]string{""},
			codes.OK,
			"",
			false,
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			if tt.exist {
				testEnv.opiSpdkServer.Volumes.NvmeVolumes[controllerName] = &controller
			}
			if tt.out != nil {
				tt.out.Name = controllerName
			}

			request := &pb.CreateNVMfRemoteControllerRequest{NvMfRemoteController: tt.in, NvMfRemoteControllerId: tt.id}
			response, err := testEnv.client.CreateNVMfRemoteController(testEnv.ctx, request)
			if response != nil {
				// if !reflect.DeepEqual(response, tt.out) {
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_NVMfRemoteControllerReset(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request without SPDK": {
			controllerID,
			&emptypb.Empty{},
			[]string{""},
			codes.OK,
			"",
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			request := &pb.NVMfRemoteControllerResetRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NVMfRemoteControllerReset(testEnv.ctx, request)
			if response != nil {
				// if !reflect.DeepEqual(response, tt.out) {
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_ListNVMfRemoteControllers(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     []*pb.NVMfRemoteController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		size    int32
		token   string
	}{
		"valid request with invalid SPDK response": {
			controllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not find any namespaces for NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
			0,
			"",
		},
		"valid request with invalid marshal SPDK response": {
			controllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevNvmeGetControllerResult"),
			true,
			0,
			"",
		},
		"valid request with empty SPDK response": {
			controllerID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "EOF"),
			true,
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			controllerID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "json response ID mismatch"),
			true,
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			controllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "json response error: myopierr"),
			true,
			0,
			"",
		},
		"valid request with valid SPDK response": {
			controllerID,
			[]*pb.NVMfRemoteController{
				{
					Name:    "OpiNvme12",
					Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
					Adrfam:  pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
					Traddr:  "127.0.0.1",
					Trsvcid: 4444,
					Subnqn:  "nqn.2016-06.io.spdk:cnode1",
					Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
				},
				{
					Name:    "OpiNvme13",
					Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
					Adrfam:  pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
					Traddr:  "127.0.0.1",
					Trsvcid: 8888,
					Subnqn:  "nqn.2016-06.io.spdk:cnode1",
					Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[` +
				`{"name":"OpiNvme13","ctrlrs":[{"state":"enabled","trid":{"trtype":"TCP","adrfam":"IPv4","traddr":"127.0.0.1","trsvcid":"8888","subnqn":"nqn.2016-06.io.spdk:cnode1"},"cntlid":1,"host":{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c","addr":"","svcid":""}}]},` +
				`{"name":"OpiNvme12","ctrlrs":[{"state":"enabled","trid":{"trtype":"TCP","adrfam":"IPv4","traddr":"127.0.0.1","trsvcid":"4444","subnqn":"nqn.2016-06.io.spdk:cnode1"},"cntlid":1,"host":{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c","addr":"","svcid":""}}]}` +
				`]}`},
			codes.OK,
			"",
			true,
			0,
			"",
		},
		"pagination overflow": {
			controllerID,
			[]*pb.NVMfRemoteController{
				{
					Name:    "OpiNvme12",
					Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
					Adrfam:  pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
					Traddr:  "127.0.0.1",
					Trsvcid: 4444,
					Subnqn:  "nqn.2016-06.io.spdk:cnode1",
					Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
				},
				{
					Name:    "OpiNvme13",
					Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
					Adrfam:  pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
					Traddr:  "127.0.0.1",
					Trsvcid: 8888,
					Subnqn:  "nqn.2016-06.io.spdk:cnode1",
					Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"OpiNvme12","ctrlrs":[{"state":"enabled","trid":{"trtype":"TCP","adrfam":"IPv4","traddr":"127.0.0.1","trsvcid":"4444","subnqn":"nqn.2016-06.io.spdk:cnode1"},"cntlid":1,"host":{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c","addr":"","svcid":""}}]},{"name":"OpiNvme13","ctrlrs":[{"state":"enabled","trid":{"trtype":"TCP","adrfam":"IPv4","traddr":"127.0.0.1","trsvcid":"8888","subnqn":"nqn.2016-06.io.spdk:cnode1"},"cntlid":1,"host":{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c","addr":"","svcid":""}}]}]}`},
			codes.OK,
			"",
			true,
			1000,
			"",
		},
		"pagination negative": {
			controllerID,
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			false,
			-10,
			"",
		},
		"pagination error": {
			controllerID,
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			false,
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			controllerID,
			[]*pb.NVMfRemoteController{
				{
					Name:    "OpiNvme12",
					Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
					Adrfam:  pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
					Traddr:  "127.0.0.1",
					Trsvcid: 4444,
					Subnqn:  "nqn.2016-06.io.spdk:cnode1",
					Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"OpiNvme12","ctrlrs":[{"state":"enabled","trid":{"trtype":"TCP","adrfam":"IPv4","traddr":"127.0.0.1","trsvcid":"4444","subnqn":"nqn.2016-06.io.spdk:cnode1"},"cntlid":1,"host":{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c","addr":"","svcid":""}}]},{"name":"OpiNvme13","ctrlrs":[{"state":"enabled","trid":{"trtype":"TCP","adrfam":"IPv4","traddr":"127.0.0.1","trsvcid":"8888","subnqn":"nqn.2016-06.io.spdk:cnode1"},"cntlid":1,"host":{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c","addr":"","svcid":""}}]}]}`},
			codes.OK,
			"",
			true,
			1,
			"",
		},
		"pagination offset": {
			controllerID,
			[]*pb.NVMfRemoteController{
				{
					Name:    "OpiNvme13",
					Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
					Adrfam:  pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
					Traddr:  "127.0.0.1",
					Trsvcid: 8888,
					Subnqn:  "nqn.2016-06.io.spdk:cnode1",
					Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"OpiNvme12","ctrlrs":[{"state":"enabled","trid":{"trtype":"TCP","adrfam":"IPv4","traddr":"127.0.0.1","trsvcid":"4444","subnqn":"nqn.2016-06.io.spdk:cnode1"},"cntlid":1,"host":{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c","addr":"","svcid":""}}]},{"name":"OpiNvme13","ctrlrs":[{"state":"enabled","trid":{"trtype":"TCP","adrfam":"IPv4","traddr":"127.0.0.1","trsvcid":"8888","subnqn":"nqn.2016-06.io.spdk:cnode1"},"cntlid":1,"host":{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c","addr":"","svcid":""}}]}]}`},
			codes.OK,
			"",
			true,
			1,
			"existing-pagination-token",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNVMfRemoteControllersRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNVMfRemoteControllers(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NvMfRemoteControllers, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NvMfRemoteControllers)
				}
				// Empty NextPageToken indicates end of results list
				if tt.size != 1 && response.NextPageToken != "" {
					t.Error("Expected end of results, receieved non-empty next page token", response.NextPageToken)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_GetNVMfRemoteController(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.NVMfRemoteController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			controllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.InvalidArgument,
			fmt.Sprintf("expecting exactly 1 result, got %v", "0"),
			true,
		},
		"valid request with invalid marshal SPDK response": {
			controllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "json: cannot unmarshal bool into Go value of type []spdk.BdevNvmeGetControllerResult"),
			true,
		},
		"valid request with empty SPDK response": {
			controllerID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			controllerID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			controllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_get_controllers: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			controllerID,
			&pb.NVMfRemoteController{
				Name:    controllerID,
				Trtype:  pb.NvmeTransportType_NVME_TRANSPORT_TCP,
				Adrfam:  pb.NvmeAddressFamily_NVMF_ADRFAM_IPV4,
				Traddr:  "127.0.0.1",
				Trsvcid: 4444,
				Subnqn:  "nqn.2016-06.io.spdk:cnode1",
				Hostnqn: "nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c",
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":[{"name":"opi-nvme8","ctrlrs":[{"state":"enabled","trid":{"trtype":"TCP","adrfam":"IPv4","traddr":"127.0.0.1","trsvcid":"4444","subnqn":"nqn.2016-06.io.spdk:cnode1"},"cntlid":1,"host":{"nqn":"nqn.2014-08.org.nvmexpress:uuid:feb98abe-d51f-40c8-b348-2753f3571d3c","addr":"","svcid":""}}]}]}`},
			codes.OK,
			"",
			true,
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeVolumes[controllerID] = &controller

			request := &pb.GetNVMfRemoteControllerRequest{Name: tt.in}
			response, err := testEnv.client.GetNVMfRemoteController(testEnv.ctx, request)
			if response != nil {
				// if !reflect.DeepEqual(response, tt.out) {
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_NVMfRemoteControllerStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with valid SPDK response": {
			controllerID,
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			[]string{""},
			codes.OK,
			"",
			false,
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Volumes.NvmeVolumes[controllerID] = &controller

			request := &pb.NVMfRemoteControllerStatsRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NVMfRemoteControllerStats(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestBackEnd_DeleteNVMfRemoteController(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		missing bool
	}{
		"valid request with invalid SPDK response": {
			controllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":false}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete Crypto: %v", controllerID),
			true,
			false,
		},
		"valid request with invalid marshal SPDK response": {
			controllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_detach_controller: %v", "json: cannot unmarshal array into Go value of type spdk.BdevNvmeDetachControllerResult"),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			controllerID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_detach_controller: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			controllerID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_detach_controller: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			controllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("bdev_nvme_detach_controller: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			controllerID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":true}`},
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			"unknown-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			false,
			false,
		},
		"unknown key with missing allowed": {
			"unknown-id",
			&emptypb.Empty{},
			[]string{""},
			codes.OK,
			"",
			false,
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			fname1 := server.ResourceIDToVolumeName(tt.in)
			testEnv.opiSpdkServer.Volumes.NvmeVolumes[controllerName] = &controller

			request := &pb.DeleteNVMfRemoteControllerRequest{Name: fname1, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNVMfRemoteController(testEnv.ctx, request)
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
			if reflect.TypeOf(response) != reflect.TypeOf(tt.out) {
				t.Error("response: expected", reflect.TypeOf(tt.out), "received", reflect.TypeOf(response))
			}
		})
	}
}