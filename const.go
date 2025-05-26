// Copyright 2025 TimeWtr
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package poolx

const (
	Running = iota
	Closed
)

const (
	_1KB = 1 << 10
	_2KB = 2 << 10
	_4KB = 4 << 10
	_8KB = 8 << 10
)

const (
	_1KBIndex = iota
	_2KBIndex
	_4KBIndex
	_8KBIndex
)

const PoolCount = 4
