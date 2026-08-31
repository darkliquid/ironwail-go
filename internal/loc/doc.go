// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

// Package loc implements Quake Enhanced / KEX string and font localization
// management, parsing localization files (e.g. loc_english.txt) and replacing
// $key tokens with translated strings and formatting placeholders.
//
// # Original C lineage
//
// Translates the localization subsystem from C Ironwail:
//   - common.h: LOC_Init, LOC_Shutdown, LOC_GetString, LOC_GetRawString, LOC_HasPlaceholders, LOC_Format
//   - common.c: LOC_LoadFile, LOC_Load, LOC_GetSystemLanguage, LOC_Language_f
package loc
