Misc Low-level utilities (tinker with aspeed2400/2500 BMC)
===========================================================

(and possibly other low-level host aspects)

pcimon
======

can display and test pcie-links of the host

ast
===
assuming BMC in non-secure, can be used to same BMC flash-image, reflash it,
observer i2c activity

asti2c
======
specialized tool to reprogram IDT chip attached to a BMC i2c bus (hardcoded)

kpoke
=====
Can dump various registers in BMC or on host (via /dev/mem or pci->bmc path)

fru
===
very limited fru decoder.



License
=======

Copyright 2019 Netflix, Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on 
an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for
 the specific language governing permissions and limitations under the License.