// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"testing"

	"golang.org/x/net/context"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/shippingservice/genproto"
)

// TestGetQuote is a check on the GetQuote RPC service including free shipping threshold.
func TestGetQuote(t *testing.T) {
	s := server{}

	baseAddress := &pb.Address{
		StreetAddress: "Muffin Man",
		City:          "London",
		State:         "",
		Country:       "England",
	}
	baseItems := []*pb.CartItem{
		{
			ProductId: "23",
			Quantity:  1,
		},
		{
			ProductId: "46",
			Quantity:  3,
		},
	}

	tests := []struct {
		name          string
		subtotal      *pb.Money
		expectedUnits int64
		expectedNanos int32
	}{
		{
			name:          "No subtotal provided (backward compatible)",
			subtotal:      nil,
			expectedUnits: 8,
			expectedNanos: 990000000,
		},
		{
			name: "Subtotal under $200 ($199.99)",
			subtotal: &pb.Money{
				CurrencyCode: "USD",
				Units:        199,
				Nanos:        990000000,
			},
			expectedUnits: 8,
			expectedNanos: 990000000,
		},
		{
			name: "Subtotal exactly $200 ($200.00)",
			subtotal: &pb.Money{
				CurrencyCode: "USD",
				Units:        200,
				Nanos:        0,
			},
			expectedUnits: 0,
			expectedNanos: 0,
		},
		{
			name: "Subtotal over $200 ($250.00)",
			subtotal: &pb.Money{
				CurrencyCode: "USD",
				Units:        250,
				Nanos:        0,
			},
			expectedUnits: 0,
			expectedNanos: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.GetQuoteRequest{
				Address:          baseAddress,
				Items:            baseItems,
				ItemsSubtotalUsd: tc.subtotal,
			}
			res, err := s.GetQuote(context.Background(), req)
			if err != nil {
				t.Fatalf("TestGetQuote (%s) failed: %v", tc.name, err)
			}
			if res.CostUsd.GetUnits() != tc.expectedUnits || res.CostUsd.GetNanos() != tc.expectedNanos {
				t.Errorf("TestGetQuote (%s): Quote value '%d.%d' does not match expected '%d.%d'",
					tc.name, res.CostUsd.GetUnits(), res.CostUsd.GetNanos(), tc.expectedUnits, tc.expectedNanos)
			}
		})
	}
}

// TestShipOrder is a basic check on the ShipOrder RPC service.
func TestShipOrder(t *testing.T) {
	s := server{}

	// A basic test case to test logic and protobuf interactions.
	req := &pb.ShipOrderRequest{
		Address: &pb.Address{
			StreetAddress: "Muffin Man",
			City:          "London",
			State:         "",
			Country:       "England",
		},
		Items: []*pb.CartItem{
			{
				ProductId: "23",
				Quantity:  1,
			},
			{
				ProductId: "46",
				Quantity:  3,
			},
		},
	}

	res, err := s.ShipOrder(context.Background(), req)
	if err != nil {
		t.Errorf("TestShipOrder (%v) failed", err)
	}
	// @todo improve quality of this test to check for a pattern such as '[A-Z]{2}-\d+-\d+'.
	if len(res.TrackingId) != 18 {
		t.Errorf("TestShipOrder: Tracking ID is malformed - has %d characters, %d expected", len(res.TrackingId), 18)
	}
}
