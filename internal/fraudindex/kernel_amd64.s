// Copyright 2026.

#include "textflag.h"

// func BlockSquaredDistance(query *[112]int16, block *[112]int16, out *[8]uint64)
//
// Per-lane squared L2 distance between two SoA blocks of 14 dims × 8 lanes.
// Layout in memory: dim d, lane l → offset (d*8 + l) * 2 bytes.
//
// Algorithm (per dimension, repeated 14 times):
//   1. Load 16 bytes of query and block (8 int16 lanes).
//   2. For each half (low 4 lanes, then high 4 lanes):
//      a. Sign-extend int16 → int32 in xmm.
//      b. Subtract to get 4 int32 deltas. |delta| ≤ 65534 fits in int32.
//      c. PMULLD self-multiply. delta² ≤ 2^32 - 1, so the low 32 bits
//         interpreted as uint32 hold the exact unsigned square.
//      d. Zero-extend two int32 squares at a time to int64 lanes and add
//         to the per-lane uint64 accumulator.
//
// Requires SSE4.1 (PMOVSXWD, PMOVZXDQ, PMULLD).
TEXT ·BlockSquaredDistance(SB), NOSPLIT, $0-24
	MOVQ query+0(FP), AX
	MOVQ block+8(FP), BX
	MOVQ out+16(FP), CX

	// Zero the four 128-bit accumulators (each holds 2 uint64 lanes).
	PXOR X10, X10
	PXOR X11, X11
	PXOR X12, X12
	PXOR X13, X13

	MOVQ $14, DX

loop:
	MOVOU (AX), X0
	MOVOU (BX), X1

	// Low 4 lanes: sign-extend low 8 bytes of X0/X1 to 4 int32.
	PMOVSXWD X0, X2
	PMOVSXWD X1, X3
	PSUBL X3, X2
	PMULLD X2, X2

	// Distribute the 4 squared int32 across two int64 accumulators.
	PMOVZXDQ X2, X4
	PSRLO $8, X2
	PMOVZXDQ X2, X5
	PADDQ X4, X10
	PADDQ X5, X11

	// High 4 lanes: shift the 16-byte loads down so high 4 int16 land in
	// the low 8 bytes, then re-run the sign-extend / subtract / square /
	// extend / accumulate pipeline.
	PSRLO $8, X0
	PSRLO $8, X1
	PMOVSXWD X0, X6
	PMOVSXWD X1, X7
	PSUBL X7, X6
	PMULLD X6, X6

	PMOVZXDQ X6, X8
	PSRLO $8, X6
	PMOVZXDQ X6, X9
	PADDQ X8, X12
	PADDQ X9, X13

	// Advance to the next dimension (16 bytes per dim per pointer).
	ADDQ $16, AX
	ADDQ $16, BX

	DECQ DX
	JNZ loop

	MOVOU X10, 0(CX)
	MOVOU X11, 16(CX)
	MOVOU X12, 32(CX)
	MOVOU X13, 48(CX)
	RET
