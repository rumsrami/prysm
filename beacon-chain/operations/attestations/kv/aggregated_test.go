package kv

import (
	"sort"
	"testing"

	fssz "github.com/ferranbt/fastssz"
	c "github.com/patrickmn/go-cache"
	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestKV_Aggregated_AggregateUnaggregatedAttestations(t *testing.T) {
	cache := NewAttCaches()
	priv, err := bls.RandKey()
	require.NoError(t, err)
	sig1 := priv.Sign([]byte{'a'})
	sig2 := priv.Sign([]byte{'b'})
	att1 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b1001}, Signature: sig1.Marshal()})
	att2 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b1010}, Signature: sig1.Marshal()})
	att3 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b1100}, Signature: sig1.Marshal()})
	att4 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b1001}, Signature: sig2.Marshal()})
	att5 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b1001}, Signature: sig1.Marshal()})
	att6 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b1010}, Signature: sig1.Marshal()})
	att7 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b1100}, Signature: sig1.Marshal()})
	att8 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b1001}, Signature: sig2.Marshal()})
	atts := []*ethpb.Attestation{att1, att2, att3, att4, att5, att6, att7, att8}
	require.NoError(t, cache.SaveUnaggregatedAttestations(atts))
	require.NoError(t, cache.AggregateUnaggregatedAttestations())

	require.Equal(t, 1, len(cache.AggregatedAttestationsBySlotIndex(1, 0)), "Did not aggregate correctly")
	require.Equal(t, 1, len(cache.AggregatedAttestationsBySlotIndex(2, 0)), "Did not aggregate correctly")
}

func TestKV_Aggregated_AggregateUnaggregatedAttestationsBySlotIndex(t *testing.T) {
	cache := NewAttCaches()
	genData := func(slot types.Slot, committeeIndex types.CommitteeIndex) *ethpb.AttestationData {
		return testutil.HydrateAttestationData(&ethpb.AttestationData{
			Slot:           slot,
			CommitteeIndex: committeeIndex,
		})
	}
	genSign := func() []byte {
		priv, err := bls.RandKey()
		require.NoError(t, err)
		return priv.Sign([]byte{'a'}).Marshal()
	}

	atts := []*ethpb.Attestation{
		// The first slot.
		{AggregationBits: bitfield.Bitlist{0b1001}, Data: genData(1, 2), Signature: genSign()},
		{AggregationBits: bitfield.Bitlist{0b1010}, Data: genData(1, 2), Signature: genSign()},
		{AggregationBits: bitfield.Bitlist{0b1100}, Data: genData(1, 2), Signature: genSign()},
		{AggregationBits: bitfield.Bitlist{0b1001}, Data: genData(1, 3), Signature: genSign()},
		{AggregationBits: bitfield.Bitlist{0b1100}, Data: genData(1, 3), Signature: genSign()},
		// The second slot.
		{AggregationBits: bitfield.Bitlist{0b1001}, Data: genData(2, 3), Signature: genSign()},
		{AggregationBits: bitfield.Bitlist{0b1010}, Data: genData(2, 3), Signature: genSign()},
		{AggregationBits: bitfield.Bitlist{0b1100}, Data: genData(2, 4), Signature: genSign()},
	}

	// Make sure that no error is produced if aggregation is requested on empty unaggregated list.
	require.NoError(t, cache.AggregateUnaggregatedAttestationsBySlotIndex(1, 2))
	require.NoError(t, cache.AggregateUnaggregatedAttestationsBySlotIndex(2, 3))
	require.Equal(t, 0, len(cache.UnaggregatedAttestationsBySlotIndex(1, 2)))
	require.Equal(t, 0, len(cache.AggregatedAttestationsBySlotIndex(1, 2)), "Did not aggregate correctly")
	require.Equal(t, 0, len(cache.UnaggregatedAttestationsBySlotIndex(1, 3)))
	require.Equal(t, 0, len(cache.AggregatedAttestationsBySlotIndex(1, 3)), "Did not aggregate correctly")

	// Persist unaggregated attestations, and aggregate on per slot/committee index base.
	require.NoError(t, cache.SaveUnaggregatedAttestations(atts))
	require.NoError(t, cache.AggregateUnaggregatedAttestationsBySlotIndex(1, 2))
	require.NoError(t, cache.AggregateUnaggregatedAttestationsBySlotIndex(2, 3))

	// Committee attestations at a slot should be aggregated.
	require.Equal(t, 0, len(cache.UnaggregatedAttestationsBySlotIndex(1, 2)))
	require.Equal(t, 1, len(cache.AggregatedAttestationsBySlotIndex(1, 2)), "Did not aggregate correctly")
	// Committee attestations haven't been aggregated.
	require.Equal(t, 2, len(cache.UnaggregatedAttestationsBySlotIndex(1, 3)))
	require.Equal(t, 0, len(cache.AggregatedAttestationsBySlotIndex(1, 3)), "Did not aggregate correctly")
	// Committee at a second slot is aggregated.
	require.Equal(t, 0, len(cache.UnaggregatedAttestationsBySlotIndex(2, 3)))
	require.Equal(t, 1, len(cache.AggregatedAttestationsBySlotIndex(2, 3)), "Did not aggregate correctly")
	// The second committee at second slot is not aggregated.
	require.Equal(t, 1, len(cache.UnaggregatedAttestationsBySlotIndex(2, 4)))
	require.Equal(t, 0, len(cache.AggregatedAttestationsBySlotIndex(2, 4)), "Did not aggregate correctly")
}

func TestKV_Aggregated_SaveAggregatedAttestation(t *testing.T) {
	tests := []struct {
		name          string
		att           *ethpb.Attestation
		count         int
		wantErrString string
	}{
		{
			name:          "nil attestation",
			att:           nil,
			wantErrString: "attestation can't be nil",
		},
		{
			name:          "nil attestation data",
			att:           &ethpb.Attestation{},
			wantErrString: "attestation's data can't be nil",
		},
		{
			name: "not aggregated",
			att: testutil.HydrateAttestation(&ethpb.Attestation{
				Data: &ethpb.AttestationData{}, AggregationBits: bitfield.Bitlist{0b10100}}),
			wantErrString: "attestation is not aggregated",
		},
		{
			name: "invalid hash",
			att: &ethpb.Attestation{
				Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
					BeaconBlockRoot: []byte{0b0},
				}),
				AggregationBits: bitfield.Bitlist{0b10111},
			},
			wantErrString: "could not tree hash attestation: " + fssz.ErrBytesLength.Error(),
		},
		{
			name: "already seen",
			att: testutil.HydrateAttestation(&ethpb.Attestation{
				Data: &ethpb.AttestationData{
					Slot: 100,
				},
				AggregationBits: bitfield.Bitlist{0b11101001},
			}),
			count: 0,
		},
		{
			name: "normal save",
			att: testutil.HydrateAttestation(&ethpb.Attestation{
				Data: &ethpb.AttestationData{
					Slot: 1,
				},
				AggregationBits: bitfield.Bitlist{0b1101},
			}),
			count: 1,
		},
	}
	r, err := hashFn(testutil.HydrateAttestationData(&ethpb.AttestationData{
		Slot: 100,
	}))
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewAttCaches()
			cache.seenAtt.Set(string(r[:]), []bitfield.Bitlist{{0xff}}, c.DefaultExpiration)
			assert.Equal(t, 0, len(cache.unAggregatedAtt), "Invalid start pool, atts: %d", len(cache.unAggregatedAtt))

			err := cache.SaveAggregatedAttestation(tt.att)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, tt.wantErrString, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.count, len(cache.aggregatedAtt), "Wrong attestation count")
			assert.Equal(t, tt.count, cache.AggregatedAttestationCount(), "Wrong attestation count")
		})
	}
}

func TestKV_Aggregated_SaveAggregatedAttestations(t *testing.T) {
	tests := []struct {
		name          string
		atts          []*ethpb.Attestation
		count         int
		wantErrString string
	}{
		{
			name: "no duplicates",
			atts: []*ethpb.Attestation{
				testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1},
					AggregationBits: bitfield.Bitlist{0b1101}}),
				testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1},
					AggregationBits: bitfield.Bitlist{0b1101}}),
			},
			count: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewAttCaches()
			assert.Equal(t, 0, len(cache.aggregatedAtt), "Invalid start pool, atts: %d", len(cache.unAggregatedAtt))
			err := cache.SaveAggregatedAttestations(tt.atts)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, tt.wantErrString, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.count, len(cache.aggregatedAtt), "Wrong attestation count")
			assert.Equal(t, tt.count, cache.AggregatedAttestationCount(), "Wrong attestation count")
		})
	}
}

func TestKV_Aggregated_AggregatedAttestations(t *testing.T) {
	cache := NewAttCaches()

	att1 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b1101}})
	att2 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b1101}})
	att3 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 3}, AggregationBits: bitfield.Bitlist{0b1101}})
	atts := []*ethpb.Attestation{att1, att2, att3}

	for _, att := range atts {
		require.NoError(t, cache.SaveAggregatedAttestation(att))
	}

	returned := cache.AggregatedAttestations()
	sort.Slice(returned, func(i, j int) bool {
		return returned[i].Data.Slot < returned[j].Data.Slot
	})
	assert.DeepEqual(t, atts, returned)
}

func TestKV_Aggregated_DeleteAggregatedAttestation(t *testing.T) {
	t.Run("nil attestation", func(t *testing.T) {
		cache := NewAttCaches()
		assert.ErrorContains(t, "attestation can't be nil", cache.DeleteAggregatedAttestation(nil))
		att := testutil.HydrateAttestation(&ethpb.Attestation{AggregationBits: bitfield.Bitlist{0b10101}, Data: &ethpb.AttestationData{Slot: 2}})
		assert.NoError(t, cache.DeleteAggregatedAttestation(att))
	})

	t.Run("non aggregated attestation", func(t *testing.T) {
		cache := NewAttCaches()
		att := testutil.HydrateAttestation(&ethpb.Attestation{AggregationBits: bitfield.Bitlist{0b1001}, Data: &ethpb.AttestationData{Slot: 2}})
		err := cache.DeleteAggregatedAttestation(att)
		assert.ErrorContains(t, "attestation is not aggregated", err)
	})

	t.Run("invalid hash", func(t *testing.T) {
		cache := NewAttCaches()
		att := &ethpb.Attestation{
			AggregationBits: bitfield.Bitlist{0b1111},
			Data: &ethpb.AttestationData{
				Slot:   2,
				Source: &ethpb.Checkpoint{},
				Target: &ethpb.Checkpoint{},
			},
		}
		err := cache.DeleteAggregatedAttestation(att)
		wantErr := "could not tree hash attestation data: " + fssz.ErrBytesLength.Error()
		assert.ErrorContains(t, wantErr, err)
	})

	t.Run("nonexistent attestation", func(t *testing.T) {
		cache := NewAttCaches()
		att := testutil.HydrateAttestation(&ethpb.Attestation{AggregationBits: bitfield.Bitlist{0b1111}, Data: &ethpb.AttestationData{Slot: 2}})
		assert.NoError(t, cache.DeleteAggregatedAttestation(att))
	})

	t.Run("non-filtered deletion", func(t *testing.T) {
		cache := NewAttCaches()
		att1 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b1101}})
		att2 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b1101}})
		att3 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 3}, AggregationBits: bitfield.Bitlist{0b1101}})
		att4 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 3}, AggregationBits: bitfield.Bitlist{0b10101}})
		atts := []*ethpb.Attestation{att1, att2, att3, att4}
		require.NoError(t, cache.SaveAggregatedAttestations(atts))
		require.NoError(t, cache.DeleteAggregatedAttestation(att1))
		require.NoError(t, cache.DeleteAggregatedAttestation(att3))

		returned := cache.AggregatedAttestations()
		wanted := []*ethpb.Attestation{att2}
		assert.DeepEqual(t, wanted, returned)
	})

	t.Run("filtered deletion", func(t *testing.T) {
		cache := NewAttCaches()
		att1 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b110101}})
		att2 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b110111}})
		att3 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b110100}})
		att4 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b110101}})
		atts := []*ethpb.Attestation{att1, att2, att3, att4}
		require.NoError(t, cache.SaveAggregatedAttestations(atts))

		assert.Equal(t, 2, cache.AggregatedAttestationCount(), "Unexpected number of atts")
		require.NoError(t, cache.DeleteAggregatedAttestation(att4))

		returned := cache.AggregatedAttestations()
		wanted := []*ethpb.Attestation{att1, att2}
		sort.Slice(returned, func(i, j int) bool {
			return string(returned[i].AggregationBits) < string(returned[j].AggregationBits)
		})
		assert.DeepEqual(t, wanted, returned)
	})
}

func TestKV_Aggregated_HasAggregatedAttestation(t *testing.T) {
	tests := []struct {
		name     string
		existing []*ethpb.Attestation
		input    *ethpb.Attestation
		want     bool
		error    bool
	}{
		{
			name:  "nil attestation",
			input: nil,
			want:  false,
			error: true,
		},
		{
			name: "nil attestation data",
			input: &ethpb.Attestation{
				AggregationBits: bitfield.Bitlist{0b1111},
			},
			want:  false,
			error: true,
		},
		{
			name: "empty cache aggregated",
			input: testutil.HydrateAttestation(&ethpb.Attestation{
				Data: &ethpb.AttestationData{
					Slot: 1,
				},
				AggregationBits: bitfield.Bitlist{0b1111}}),
			want: false,
		},
		{
			name: "empty cache unaggregated",
			input: testutil.HydrateAttestation(&ethpb.Attestation{
				Data: &ethpb.AttestationData{
					Slot: 1,
				},
				AggregationBits: bitfield.Bitlist{0b1001}}),
			want: false,
		},
		{
			name: "single attestation in cache with exact match",
			existing: []*ethpb.Attestation{{
				Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
					Slot: 1,
				}),
				AggregationBits: bitfield.Bitlist{0b1111}},
			},
			input: &ethpb.Attestation{
				Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
					Slot: 1,
				}),
				AggregationBits: bitfield.Bitlist{0b1111}},
			want: true,
		},
		{
			name: "single attestation in cache with subset aggregation",
			existing: []*ethpb.Attestation{{
				Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
					Slot: 1,
				}),
				AggregationBits: bitfield.Bitlist{0b1111}},
			},
			input: &ethpb.Attestation{
				Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
					Slot: 1,
				}),
				AggregationBits: bitfield.Bitlist{0b1110}},
			want: true,
		},
		{
			name: "single attestation in cache with superset aggregation",
			existing: []*ethpb.Attestation{{
				Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
					Slot: 1,
				}),
				AggregationBits: bitfield.Bitlist{0b1110}},
			},
			input: &ethpb.Attestation{
				Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
					Slot: 1,
				}),
				AggregationBits: bitfield.Bitlist{0b1111}},
			want: false,
		},
		{
			name: "multiple attestations with same data in cache with overlapping aggregation, input is subset",
			existing: []*ethpb.Attestation{
				{
					Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
						Slot: 1,
					}),
					AggregationBits: bitfield.Bitlist{0b1111000},
				},
				{
					Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
						Slot: 1,
					}),
					AggregationBits: bitfield.Bitlist{0b1100111},
				},
			},
			input: &ethpb.Attestation{
				Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
					Slot: 1,
				}),
				AggregationBits: bitfield.Bitlist{0b1100000}},
			want: true,
		},
		{
			name: "multiple attestations with same data in cache with overlapping aggregation and input is superset",
			existing: []*ethpb.Attestation{
				{
					Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
						Slot: 1,
					}),
					AggregationBits: bitfield.Bitlist{0b1111000},
				},
				{
					Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
						Slot: 1,
					}),
					AggregationBits: bitfield.Bitlist{0b1100111},
				},
			},
			input: &ethpb.Attestation{
				Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
					Slot: 1,
				}),
				AggregationBits: bitfield.Bitlist{0b1111111}},
			want: false,
		},
		{
			name: "multiple attestations with different data in cache",
			existing: []*ethpb.Attestation{
				{
					Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
						Slot: 2,
					}),
					AggregationBits: bitfield.Bitlist{0b1111000},
				},
				{
					Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
						Slot: 3,
					}),
					AggregationBits: bitfield.Bitlist{0b1100111},
				},
			},
			input: &ethpb.Attestation{
				Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
					Slot: 1,
				}),
				AggregationBits: bitfield.Bitlist{0b1111111}},
			want: false,
		},
		{
			name: "attestations with different bitlist lengths",
			existing: []*ethpb.Attestation{
				{
					Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
						Slot: 2,
					}),
					AggregationBits: bitfield.Bitlist{0b1111000},
				},
			},
			input: &ethpb.Attestation{
				Data: testutil.HydrateAttestationData(&ethpb.AttestationData{
					Slot: 2,
				}),
				AggregationBits: bitfield.Bitlist{0b1111},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewAttCaches()
			require.NoError(t, cache.SaveAggregatedAttestations(tt.existing))

			if tt.input != nil && tt.input.Signature == nil {
				tt.input.Signature = make([]byte, 96)
			}

			if tt.error == true {
				_, err := cache.HasAggregatedAttestation(tt.input)
				require.ErrorContains(t, "can't be nil", err)
			} else {
				result, err := cache.HasAggregatedAttestation(tt.input)
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)

				// Same test for block attestations
				cache = NewAttCaches()
				assert.NoError(t, cache.SaveBlockAttestations(tt.existing))

				result, err = cache.HasAggregatedAttestation(tt.input)
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestKV_Aggregated_DuplicateAggregatedAttestations(t *testing.T) {
	cache := NewAttCaches()

	att1 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b1101}})
	att2 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b1111}})
	atts := []*ethpb.Attestation{att1, att2}

	for _, att := range atts {
		require.NoError(t, cache.SaveAggregatedAttestation(att))
	}

	returned := cache.AggregatedAttestations()

	// It should have only returned att2.
	assert.DeepSSZEqual(t, att2, returned[0], "Did not receive correct aggregated atts")
	assert.Equal(t, 1, len(returned), "Did not receive correct aggregated atts")
}
