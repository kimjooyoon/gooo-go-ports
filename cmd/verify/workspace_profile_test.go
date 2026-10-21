package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Fixed expectations from the independently reviewed immutable inventory at535b931.
// Inventory SHA256:dff5a73c63cfebf5517b13739bff6115287889f4ee3c4c824644cc34beaca624.
// These are literal expected manifests/rosters, not values produced by the selector.
var workspaceProfileReviewed70 = []struct {
	entry                                          string
	cases, currentBatches                          int
	changed                                        bool
	defaultManifest, closureManifest, orderedNames string
}{
	{"go/net/http:StatusText", 625, 5, true, "sha256:073d62b00516a196f84c7431e19dd050141a27ea66e5cafc97ff9450fee8fc90", "sha256:d72f09dd5c438d3f5f43045e2e858722456ea7f8bee16aca5aa76ade00587869", "ports/net/http.gooo"},
	{"go/unicode/utf8:ValidRune", 162, 2, true, "sha256:38964d355f5388f8a032f6009044968af21fa68c14a55720fab22a0adaf92f48", "sha256:70697764b1db74ebc387c110a74907ce057e5a592b97532ccf631417c3cee561", "ports/unicode/utf8.gooo"},
	{"go/unicode/utf8:RuneLen", 162, 2, true, "sha256:1da0e870c955f239de5337b9e832dea820b8f0c38eeb221430e7e3c76e2b68d3", "sha256:ae3ada83f7c6da1d3a82a4e77d95431d50f8c52e450566964711190a6ca40446", "ports/unicode/utf8.gooo"},
	{"go/unicode/utf8:RuneStart", 256, 2, true, "sha256:94edb7b7f6d49bc895bd2ab632c96ab8b27b2042e3a35686265555e6535bdf62", "sha256:2dc8776da163d8f68e1ac5339a76139a90478cb5ac967e234b6b73c0800f38f8", "ports/unicode/utf8.gooo"},
	{"go/unicode/utf16:IsSurrogate", 162, 2, true, "sha256:87219b84c6d93e8b08a4eec2e6f32c1ea6fce5fb20b7650830a3019db3852bf5", "sha256:4edaeda3d03eff9d4b3d8e323fd17ed631565b75ee27a3f80138ed936c6ff6ab", "ports/unicode/utf16.gooo"},
	{"go/unicode/utf16:RuneLen", 162, 2, true, "sha256:68bf840fafe9b1944605e2879fc72ea2687e616239cee9de03b2612d60efed4c", "sha256:a84b6e65b375f919b459836bf8b4778559ec751ea510da0d4064f12cb8826d92", "ports/unicode/utf16.gooo"},
	{"go/unicode/utf16:EncodeRune", 162, 2, true, "sha256:e96aa3c6f4b19c24afbec2b7777776ff6335b13f0da3e0f64cb4c93475efa8d0", "sha256:5a27d88461e1f48ee9f06987b897eed6053a2b8e962a3065972bee271d0431d8", "ports/unicode/utf16.gooo"},
	{"go/unicode/utf16:DecodeRune", 164, 2, true, "sha256:05a400145fa05d014f41d5418b5217d32ad9f616c1263260f8445ca5fa07ed92", "sha256:7a9ab6b0bc76d7c290437e1a7c17641316b16891cbfb319ea453047014d0aecd", "ports/unicode/utf16.gooo"},
	{"app/response:Describe", 6, 1, true, "sha256:ae96e5c038dbc0f27fe0eeaf36864f5a3c962e64068271740c6b1bd116b96410", "sha256:0f8cac91b2d3beacd7c95c592dc80e5b2c2a127b70fcd9061d286f78e5fffb96", "ports/net/http.gooo|examples/response.gooo"},
	{"go/math/bits:Len8", 256, 2, true, "sha256:68663b8fdc71cb562a815591f964bcefbeef383612082730705b8bbf3f849eb7", "sha256:561483e2cda0d67d232ec13e591f8ed74c29cea31047a315f92b686934b02f46", "ports/math/bits.gooo"},
	{"go/math/bits:LeadingZeros8", 256, 2, true, "sha256:f9ef73f452e29d05886b322239389e8241cf1483d055894e1f7daa515ede44ed", "sha256:d325648c1d0709bf30d9e4018eec5bc9616d58d30b0bab57c752079a55719a22", "ports/math/bits.gooo"},
	{"go/math/bits:TrailingZeros8", 256, 2, true, "sha256:225f8d216fddb9e46279065751763f880536268572082cb3b4e96f9390521dd0", "sha256:1dc877f218190e73522dc5bc6992a26dd620af51485cfe15ec3f5e92a1a6095d", "ports/math/bits.gooo"},
	{"go/math/bits:OnesCount8", 256, 2, true, "sha256:a041fd9ef90ef02f6ca79ec967552922e943c16c4be9b4ad43834bf9533adbb4", "sha256:a9ed7e6b75d14cf4c6af2a3245e626fe61d7c21baaa8b65a1885596744a2a78b", "ports/math/bits.gooo"},
	{"go/math/bits:Reverse8", 256, 2, true, "sha256:9680e2b0e1d53e96a0d4c60f1ddccb4f1c8ac8113c442e761542dd1fff77ad2d", "sha256:456ecaea62d823789537f0fd5fc51b9ec77706c0bcc82a19b589254aeaec8ad6", "ports/math/bits.gooo"},
	{"app/byte-summary:ByteSummary", 6, 1, true, "sha256:dcc6083386ce7494f79f871c01f6eacf603d1cf1e2954855e53fe3fe5e04daec", "sha256:1b6ab216b2e4de58af4913e515d3669bceec0b2b450822250e14acc15151c5da", "ports/math/bits.gooo|examples/byte-summary/main.gooo"},
	{"app/unicode-inspector:InspectScalar", 168, 3, true, "sha256:826a72690f92b5deff8dc54e5974f3b201cef641a372efbe20edaee8ae772625", "sha256:6bfbb094431d92803685cdfdae1339445abafa260ff9f0c5b2280be46b4ca476", "ports/unicode/utf8.gooo|ports/unicode/utf16.gooo|examples/unicode-inspector/main.gooo"},
	{"app/unicode-inspector:InspectPair", 178, 3, true, "sha256:130409d183145d5e8c17b416228090b6353ac3cee377fa3c9ff14e7feb002c99", "sha256:55ef6708031a42db39a294409b9e80f5c05b107355b757194ddde2cb8d6f4459", "ports/unicode/utf8.gooo|ports/unicode/utf16.gooo|examples/unicode-inspector/main.gooo"},
	{"go/image/color:RGBToCMYK", 661, 11, true, "sha256:b6ebb8ff908f44bf3fcec019fe6f6ac22e128364fe2d26546541a237609d2007", "sha256:f2840c41e02791a1fbfea4290a935c9de9ab661c6b4a2c248bb3695889c23983", "ports/image/color.gooo|ports/image/ycbcr.gooo|ports/image/nrgba.gooo"},
	{"go/image/color:CMYKToRGB", 695, 11, true, "sha256:59cdec2a988a579a7d2f7ce7df3cfc88e607589e1677891eaf8ce25a4664bb67", "sha256:64bb68b7c0839bb2f057edc09eaf7b7ce57ffef679db2d3337d504c8a755eee5", "ports/image/color.gooo|ports/image/ycbcr.gooo|ports/image/nrgba.gooo"},
	{"app/color-inspector:InspectRGB", 661, 21, true, "sha256:f3ee4449c262b92e2ef6e5257bf07194775eef6e0bbebe899a9df8287269fcff", "sha256:8665fadd03ecd7a6f7c1abeabd83d1ab4efddfd06b5e2b145e1b709f132cc50b", "ports/image/color.gooo|ports/image/ycbcr.gooo|ports/image/nrgba.gooo|examples/color-inspector/main.gooo"},
	{"go/math/bits:RotateLeft8", 2508, 20, true, "sha256:4d09b5492c977c5072a103d22df682e20fceb5cb4c3f72ce7e7c88ebbc8444fe", "sha256:0ea43e178284f42fc824c5d48cd71a7587f2fe071b560a733407d81410ba0620", "ports/math/bits.gooo"},
	{"app/byte-rotation:Inspect", 708, 12, true, "sha256:03c663001a9c66896b6de1e7105939c02b3b938307e1bea5effa00b7683863d2", "sha256:78df753df26d28d6a77278e4bfefaabba71f17905771833869bcfd07cf837a12", "ports/math/bits.gooo|examples/byte-rotation/main.gooo"},
	{"go/time:DurationRound", 1789, 14, true, "sha256:2cdc36606d5b87ab3a75350f9c7059d52e91a241a9f84788e30c3b0e99c145ce", "sha256:98bea7889901a19b7e1d938d87116e47ae9f9602716d107f5cd0822ce48eecef", "ports/time/duration.gooo"},
	{"go/time:DurationTruncate", 1789, 14, true, "sha256:8b4b173ffd44ba66b6d780b691957b8ea82643da05741fb048f55cc9739f1416", "sha256:ca30ef2fb3283195489ac2236e22c0e51998f208e3c89fe7e3e7eac0d63e8cbb", "ports/time/duration.gooo"},
	{"app/duration-quantizer:Inspect", 1789, 28, true, "sha256:470e54c0bf35ae11b55270cbe6f96fd44010c91340ca8d65547782a24fb04c65", "sha256:aa02af305b339ef595389979e547b5949ac1cdb551d7f941a7a0ba6249239a3b", "ports/time/duration.gooo|examples/duration-quantizer/main.gooo"},
	{"go/unicode:IsSpace", 514, 5, true, "sha256:0b59ea72bfdd0c8a48051af59eeb77133ec99242eb66af70d70d70da1e0938b1", "sha256:738b0e068008032dabb8a191578a4b26750d8fec519cac95f8faccd01e9a41a3", "ports/unicode/space.gooo|ports/unicode/classification.gooo|ports/unicode/simple_fold.gooo|ports/unicode/case_mapping.gooo"},
	{"app/whitespace-inspector:Inspect", 514, 9, true, "sha256:0651ae30de3f46083427879ed7d28c78632d84749bab139f8d05b28f0349825c", "sha256:e179f03ac00e3c496844b505a35c4a315f518cda5aec142d596a9b3cdc8aa004", "ports/unicode/utf8.gooo|ports/unicode/utf16.gooo|ports/unicode/space.gooo|ports/unicode/classification.gooo|ports/unicode/simple_fold.gooo|ports/unicode/case_mapping.gooo|examples/whitespace-inspector/main.gooo"},
	{"go/unicode:IsLetter", 4140, 33, true, "sha256:333e0b9d5655a6351b0da1ad6c157f8c54f0bc2b144e21dc8404783b3404e250", "sha256:2a1e775992463593d868484dae94736af255121abdff5875f134d72e45582c5b", "ports/unicode/space.gooo|ports/unicode/classification.gooo|ports/unicode/simple_fold.gooo|ports/unicode/case_mapping.gooo"},
	{"go/unicode:IsDigit", 927, 8, true, "sha256:57160be3bb722a377e0d7afa691675f17a7176250c07a917086f968eb8f2feb2", "sha256:fd0007245c1c0e0b464fa3622fe409a82ef38bb98178e9864861b3d79bd89061", "ports/unicode/space.gooo|ports/unicode/classification.gooo|ports/unicode/simple_fold.gooo|ports/unicode/case_mapping.gooo"},
	{"app/identifier-rune:Classify", 4582, 72, true, "sha256:72b6a40b8ed6cadbc3ae5f0eb1d5aeaa38ddaf842ad0bd4ce486e9a9551def3f", "sha256:6241847e6973451f2b4f70ddc8e301af36c5e5b374777ac3653bea8c645f2e4d", "ports/unicode/space.gooo|ports/unicode/classification.gooo|ports/unicode/simple_fold.gooo|ports/unicode/case_mapping.gooo|examples/identifier-rune/main.gooo"},
	{"go/image:RectangleEmpty", 253, 4, true, "sha256:ab52a3099744389d6450526d2fc21e3869a4505dabec4f91a6a525b41543cef2", "sha256:9d068c3bc543a4ca24d78129c25cef6160e79c44d0e9aa6cfe0e85461d3cd9c5", "ports/image/geometry.gooo|ports/image/canon.gooo"},
	{"go/image:RectangleOverlaps", 486, 16, true, "sha256:2452cc1e59bf3ee1e6f66f7180ce840509f6acea36d9a24702f90dff98c59119", "sha256:75eae0c1f4147dd55cc30b4e53527122253c63341a90f5131e31b2c0d55423d2", "ports/image/geometry.gooo|ports/image/canon.gooo"},
	{"go/image:RectangleIntersect", 486, 16, true, "sha256:1432f8a23b34d44efb6a5a573952bbcd33bafe98c679113604f79ceaa2720456", "sha256:3110dd85a84795557af6049d59b8d85e62141bfdb86d4a437ef2b6387bc07c43", "ports/image/geometry.gooo|ports/image/canon.gooo"},
	{"go/image:RectangleUnion", 486, 16, true, "sha256:651b5a2d82f37fdf983f6dd062267ba25f14db0e63e1cd1975a17cf252d065ae", "sha256:3ce1f0205d73eab502c4234bb99326c04492cb9a1668bfd1329ea97209bd115b", "ports/image/geometry.gooo|ports/image/canon.gooo"},
	{"example/crop-planner:CropPlan", 486, 16, true, "sha256:0ea158b6f918d679b361063544404032045d1a4b7981cc628e0b87df2a31e43a", "sha256:a925e30f6c4e760e3bba14dc31e66e7a43609d1b596873ecde244f771d8680f8", "ports/image/geometry.gooo|ports/image/canon.gooo|examples/crop-planner/crop_planner.gooo|examples/crop-planner/drag_crop.gooo"},
	{"go/unicode:SimpleFold", 3516, 28, true, "sha256:0b89fc6be33d4df943063ba02e6ed1a6063e3a81c89fb5eeace7d80d639d49ad", "sha256:ddff0e217e84b3c457916cd9a03e99de04aca29c175d0c73bd6c454fd56529cb", "ports/unicode/space.gooo|ports/unicode/classification.gooo|ports/unicode/simple_fold.gooo|ports/unicode/case_mapping.gooo"},
	{"app/simple-fold:Compare", 531, 9, true, "sha256:e6fc20cb40fdaf8fcc3a613f2d515a2df643226ed95ef89c6bd7e116f212bf02", "sha256:bbb5e956f13b14ca717aa98eff2321b073be769ccc8008b57b56f76b5124189d", "ports/unicode/space.gooo|ports/unicode/classification.gooo|ports/unicode/simple_fold.gooo|ports/unicode/case_mapping.gooo|examples/simple-fold/main.gooo"},
	{"go/image/color:RGBToYCbCr", 948, 15, true, "sha256:d3b1f9ca247a2fd400ca38f54734c3057b56aac29183599d964a03ef3a35fcf5", "sha256:79f4425930909cf87d5f9a34239a0bcb65f61d8397e4a69beec8ee1de8ce8457", "ports/image/color.gooo|ports/image/ycbcr.gooo|ports/image/nrgba.gooo"},
	{"go/image/color:YCbCrToRGB", 1260, 20, true, "sha256:377778893bc0e2b59a54f24a9cb5355bb406cc1d29d90d3b09b9303a6b7ff4c0", "sha256:4d174b83da3f98e3a6dd6e1b3f8f5bacf6733622f82855a1be60e23fc997c808", "ports/image/color.gooo|ports/image/ycbcr.gooo|ports/image/nrgba.gooo"},
	{"app/ycbcr-inspector:InspectRGB", 948, 30, true, "sha256:12379e14e1030a9cbdae8225890086fe4203eb0bf15dd90eb0ba68abbadbf328", "sha256:84d8538fec26dba5794112428a1e64b566295cad6dcbcdba892ca1febe232031", "ports/image/color.gooo|ports/image/ycbcr.gooo|ports/image/nrgba.gooo|examples/ycbcr-inspector/main.gooo"},
	{"go/image:RectangleCanon", 371, 6, true, "sha256:ed4f1ec8b1bf757102705b90ace114e71f8a65614a423b17c7a2a9128b718e1f", "sha256:50c399f18faa5d5271a48f3f815e45c1283384976892b41d9bc56555a53c6e15", "ports/image/geometry.gooo|ports/image/canon.gooo"},
	{"example/crop-planner:DragCropPlan", 501, 16, true, "sha256:5625dccff7c466fd7be11cdbc445936e8f3c99a4bb0a528f30779ad0a89dab19", "sha256:51e74b1bc8dc53d28827df1c64946dfd065403701c90df9677d1aa4d4702fbb0", "ports/image/geometry.gooo|ports/image/canon.gooo|examples/crop-planner/crop_planner.gooo|examples/crop-planner/drag_crop.gooo"},
	{"go/golang.org/x/image/math/fixed:Int26_6Floor", 840, 14, true, "sha256:33ada087177e57e4cb067f8b70931b906d55f8306a55c4f5d9e8929437707dd9", "sha256:0c740c95fd71bfc07d76924f1f1344d48c51cbccf8032ac7a49c16143d5aeed9", "ports/image/fixed.gooo"},
	{"go/golang.org/x/image/math/fixed:Int26_6Ceil", 840, 14, true, "sha256:05d472dd72c7672ce082c3d2b05b4b8c584835ab3a202d95ee5b298021788f64", "sha256:549916eb0fa2476715076ceff215c1cf896d2aaceb149756808c80f99382a912", "ports/image/fixed.gooo"},
	{"example/fixed-pixel-bounds:PixelBounds", 510, 16, true, "sha256:713541a798e3c15b5f7f1f96058400604431869a8b2c0b6ead1afc8db6e1d7da", "sha256:f8e95c3303fa53c28c3e74cc46c2ac3e5fa0292677ce94bc7527f57971bf52b5", "ports/image/geometry.gooo|ports/image/canon.gooo|ports/image/fixed.gooo|examples/fixed-pixel-bounds/main.gooo"},
	{"go/unicode:To", 8335, 66, true, "sha256:6f199f9f645795a852089d44d73cff1db8ab0dea8bf5366ed0cec7b7d004a44b", "sha256:8ffb78d7365f7d840843df8118bec92e730c614ed9565157876bc966fde2e026", "ports/unicode/space.gooo|ports/unicode/classification.gooo|ports/unicode/simple_fold.gooo|ports/unicode/case_mapping.gooo"},
	{"go/unicode:ToUpper", 2702, 22, true, "sha256:74c3c85dddb966a56480a40faf51c76a74795fb2df9c2035ff4cd20560e65bdf", "sha256:77ef56711ffbd2e43fb73bb8ad2530911d863c08a8675f5159dc40bd07362cfc", "ports/unicode/space.gooo|ports/unicode/classification.gooo|ports/unicode/simple_fold.gooo|ports/unicode/case_mapping.gooo"},
	{"go/unicode:ToLower", 2702, 22, true, "sha256:9e390260e8597ef5a97554dae51f92735e99085c5ab814848b07f6ae51f284a7", "sha256:6d7d29aac843278042710935b57db1e0de7691e490e9dceccfcf2dfd073d54ac", "ports/unicode/space.gooo|ports/unicode/classification.gooo|ports/unicode/simple_fold.gooo|ports/unicode/case_mapping.gooo"},
	{"go/unicode:ToTitle", 2667, 21, true, "sha256:a30535ade7393cfaa0772ef488d2df71c14a2fa2c97742a83fd3702cb5eb63fe", "sha256:dcbdded8898349c27cd3790b0d1f9c46cf766590acc1dee77cc7c59182c62439", "ports/unicode/space.gooo|ports/unicode/classification.gooo|ports/unicode/simple_fold.gooo|ports/unicode/case_mapping.gooo"},
	{"app/case-inspector:Describe", 3613, 57, true, "sha256:a3ee373951375966f5f98377db3215751c7c18593adc85a2abfb27cf37716b42", "sha256:78f85463b6512a07faed7cb5f2405d65bc392b694879fcfc0e1de124a523875b", "ports/unicode/utf8.gooo|ports/unicode/space.gooo|ports/unicode/classification.gooo|ports/unicode/simple_fold.gooo|ports/unicode/case_mapping.gooo|examples/case-inspector/main.gooo"},
	{"go/image/color:NRGBAToRGBA", 2728, 43, true, "sha256:7175f075b298c3ea4125a966f27ccaa5a6b885730824489dc2a9c4d5c155e882", "sha256:d2edcefa7cfcbb84e481d64eb83bf9868bfbdfd5c7a02d705e7ed2c71bf0e4af", "ports/image/color.gooo|ports/image/ycbcr.gooo|ports/image/nrgba.gooo"},
	{"app/alpha-matte:MattePixel", 2262, 71, true, "sha256:1cea76327c0990eb097d66c59e3ceedb5bd3e0336fbf18a6dade449f92bde6fa", "sha256:933cd5779a5013c627ad43e427bb605b4be08a405a5fa476f839d86171d58eff", "ports/image/color.gooo|ports/image/ycbcr.gooo|ports/image/nrgba.gooo|examples/alpha-matte/main.gooo"},
	{"go/strings:IndexByte", 1552, 21, true, "sha256:89d9c68f00384951ae06b328d6e0af9f43173eecf0d4472f752854cc21e0815d", "sha256:f1db9b10c38e68e13eba1af0715aad45264ea31491b3daa5f36c4a07e943ec38", "ports/strings/search.gooo"},
	{"app/string-byte-search:Search", 1552, 28, true, "sha256:ead80d3b526ba8544e174244499e776cf07b70b9bdd544840194ede4d2bec95d", "sha256:fde07cdc2d381a1d8a391e08325deb1f7b9ae885c8ba9159af78c73ef44c9a83", "ports/strings/search.gooo|examples/string-byte-search/main.gooo"},
	{"go/github.com/google/uuid:Validate", 21745, 195, false, "sha256:88d6795b23c25c7cbb8e2bde221bf5a20a89367179a6d105797118de4b6e41af", "sha256:88d6795b23c25c7cbb8e2bde221bf5a20a89367179a6d105797118de4b6e41af", "ports/google/uuid/quote.gooo|ports/google/uuid/validate.gooo"},
	{"app/uuid-format-inspector:Inspect", 21745, 197, false, "sha256:b05a90e7ee3935da94a497a403d4d2ad4f5bb8616a9d1e5e6563e99f3b893892", "sha256:b05a90e7ee3935da94a497a403d4d2ad4f5bb8616a9d1e5e6563e99f3b893892", "ports/google/uuid/quote.gooo|ports/google/uuid/validate.gooo|examples/uuid-format-inspector/main.gooo"},
	{"go/github.com/google/uuid/text:ParseCanonical", 21867, 239, false, "sha256:bd93ecf0ef0d84fb37bb0497c5ad13a40a93d8c32f3c610e18a3f4b4d6817f7d", "sha256:bd93ecf0ef0d84fb37bb0497c5ad13a40a93d8c32f3c610e18a3f4b4d6817f7d", "ports/google/uuid/quote.gooo|ports/google/uuid/validate.gooo|ports/google/uuid/text/parse_canonical.gooo"},
	{"go/image:RectangleIn", 570, 18, true, "sha256:781f75f46837d999bab87014812fcf27304bf2b2e7cbae86474544482fa5e77e", "sha256:021f5394bb04c8114308083ae1e1d22c5a196a3f85871a466f49b2f536a55344", "ports/image/geometry.gooo|ports/image/canon.gooo"},
	{"example/fits-crop:FitsCrop", 570, 18, true, "sha256:81e2afc910ca2d28d47c42f15bfb0a127db6e8ef4e5a61246ed2005b96f5883e", "sha256:9a6ed79b1946afb69e7fabdcf6af6cfa4c560fc48c6ed4fcb2d92ab52197d417", "ports/image/geometry.gooo|ports/image/canon.gooo|examples/fits-crop/main.gooo"},
	{"go/strings:Index", 9313, 137, false, "sha256:b8d3c0ebc0df37b981242ecaf024b4b9a3b40141891abba26abde9feedefc373", "sha256:b8d3c0ebc0df37b981242ecaf024b4b9a3b40141891abba26abde9feedefc373", "ports/strings/search.gooo|ports/strings/literal.gooo"},
	{"go/strings:Count", 9313, 137, false, "sha256:93a9feeea472e2bc77f12719646f1f27a8eca3ee2e3e3a94f72c6c47dd956c33", "sha256:93a9feeea472e2bc77f12719646f1f27a8eca3ee2e3e3a94f72c6c47dd956c33", "ports/strings/search.gooo|ports/strings/literal.gooo"},
	{"app/literal-text-report:Report", 9313, 235, false, "sha256:ade8bd95ab52c12767207270e63cba1cc7a81721e7641222448c79cbfdfb8c88", "sha256:ade8bd95ab52c12767207270e63cba1cc7a81721e7641222448c79cbfdfb8c88", "ports/strings/search.gooo|ports/strings/literal.gooo|examples/literal-text-report/main.gooo"},
	{"go/strings:Cut", 32, 1, false, "sha256:c5fdd2291365b3f41d1126e90f89119c95b96417713460871c33c684d739d6f9", "sha256:c5fdd2291365b3f41d1126e90f89119c95b96417713460871c33c684d739d6f9", "ports/strings/search.gooo|ports/strings/literal.gooo|ports/strings/cut.gooo"},
	{"go/strings:CutPrefix", 32, 1, false, "sha256:7beb50fec4e8ec0d9ea8c8f7fca3ad40bccaf25ca063599ed8e8c104147a8d04", "sha256:7beb50fec4e8ec0d9ea8c8f7fca3ad40bccaf25ca063599ed8e8c104147a8d04", "ports/strings/search.gooo|ports/strings/literal.gooo|ports/strings/cut.gooo"},
	{"go/strings:CutSuffix", 32, 1, false, "sha256:fe87e1fe345c431f461811ab44600a9cb7abe6654726b7702da069a28314fc5d", "sha256:fe87e1fe345c431f461811ab44600a9cb7abe6654726b7702da069a28314fc5d", "ports/strings/search.gooo|ports/strings/literal.gooo|ports/strings/cut.gooo"},
	{"app/literal-cut-report:Report", 32, 2, false, "sha256:50f590a9d5df7fdf9f0865766e77692353b353eadee9b2a50ec66ad6d864551c", "sha256:50f590a9d5df7fdf9f0865766e77692353b353eadee9b2a50ec66ad6d864551c", "ports/strings/search.gooo|ports/strings/literal.gooo|ports/strings/cut.gooo|examples/literal-cut-report/main.gooo"},
	{"go/strconv:ParseIntDecimal64", 256, 5, false, "sha256:5eed603cef8f70cda4ab5d48916730db5419abbaacf3f3f8a32b910babffd6bc", "sha256:5eed603cef8f70cda4ab5d48916730db5419abbaacf3f3f8a32b910babffd6bc", "ports/strconv/decimal.gooo"},
	{"app/payload-admission:Report", 101, 4, false, "sha256:c238a35726b6040d2d47c73a2a840657115fe24b4f3c24fb7c93949177a34d8e", "sha256:c238a35726b6040d2d47c73a2a840657115fe24b4f3c24fb7c93949177a34d8e", "ports/strconv/decimal.gooo|examples/payload-admission/main.gooo"},
	{"go/path:Clean", 9841, 77, false, "sha256:8bd556e32b7d875ae8c7558ec4bb728c8f29abe5bde4ea4df0a7967ad1d14cf8", "sha256:8bd556e32b7d875ae8c7558ec4bb728c8f29abe5bde4ea4df0a7967ad1d14cf8", "ports/path/path.gooo|ports/path/clean.gooo"},
	{"app/path-alias-facts:Report", 120, 4, false, "sha256:4085c9d443151b367d3c88b85cb62195431f5fc05751cb80a1af87f4b608065c", "sha256:4085c9d443151b367d3c88b85cb62195431f5fc05751cb80a1af87f4b608065c", "ports/path/path.gooo|ports/path/clean.gooo|examples/path-alias-facts/main.gooo"},
}

func workspaceProfilePinnedLock() upstream {
	return upstream{CompilerRevision: "205896a413e5dea92366309c35ae85e542d055f6", CompilerVersion: "0.6.37-dev", GoVersion: "go1.27.3"}
}

func TestNativeWorkspaceProfileParserPinsAndEarlyRefusal(t *testing.T) {
	for _, browser := range []bool{false, true} {
		for _, value := range []string{"", "legacy"} {
			if got, err := parseNativeWorkspaceProfile(value, browser); err != nil || got != "" {
				t.Fatal("legacy workspace alias changed", value, browser, got, err)
			}
		}
	}
	if got, err := parseNativeWorkspaceProfile("dependency-closure-v1", false); err != nil || got != dependencyClosureWorkspaceProfile {
		t.Fatal("explicit workspace profile refused", got, err)
	}
	if _, err := parseNativeWorkspaceProfile("dependency-closure-v1", true); err == nil {
		t.Fatal("native workspace profile affected browser fixtures")
	}
	for _, value := range []string{"dependency-closure-v2", "DEPENDENCY-CLOSURE-V1", " dependency-closure-v1", "dependency-closure-v1 ", "unknown"} {
		if _, err := parseNativeWorkspaceProfile(value, false); err == nil {
			t.Fatal("unknown workspace profile accepted", value)
		}
	}
	if err := nativeWorkspaceProfile("unrecognized").validate(); err == nil {
		t.Fatal("directly constructed unknown workspace profile accepted")
	}
	if err := dependencyClosureWorkspaceProfile.validateCompiler(workspaceProfilePinnedLock()); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"revision", "version", "Go"} {
		lock := workspaceProfilePinnedLock()
		switch field {
		case "revision":
			lock.CompilerRevision = strings.Repeat("0", 40)
		case "version":
			lock.CompilerVersion = "0.6.34-dev"
		case "Go":
			lock.GoVersion = "go1.27.1"
		}
		if err := dependencyClosureWorkspaceProfile.validateCompiler(lock); err == nil {
			t.Fatal("workspace profile accepted a different compiler/toolchain", field)
		}
		if err := nativeWorkspaceProfile("").validateCompiler(lock); err != nil {
			t.Fatal("legacy behavior acquired new pin restrictions", field, err)
		}
	}
	// No upstream.json or compiler exists here. Both failures must precede all
	// pin/output work, without creating even the requested output parent.
	t.Chdir(t.TempDir())
	priorFlags, priorArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = priorFlags, priorArgs })
	for _, test := range []struct{ name, option, value, reason string }{
		{"unknown", "--out", "unknown", "unknown --workspace-profile"},
		{"browser", "--browser-fixtures", "dependency-closure-v1", "browser"},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := filepath.Join("uncreated-"+test.name, "output")
			flag.CommandLine = flag.NewFlagSet("workspace-profile-test", flag.ContinueOnError)
			flag.CommandLine.SetOutput(&bytes.Buffer{})
			os.Args = []string{"verify", test.option, target, "--workspace-profile", test.value}
			if err := run(); err == nil || !strings.Contains(err.Error(), test.reason) {
				t.Fatal("workspace flag validation did not precede I/O", test.name, err)
			}
			if _, err := os.Stat(filepath.Dir(target)); !os.IsNotExist(err) {
				t.Fatal("refused workspace profile created output parent", err)
			}
		})
	}
}

func TestWorkspaceProfileMetadataIsIndependentAndLegacyBytesStayAbsent(t *testing.T) {
	zero := nativeWorkspaceProfile("")
	if zero.identity() != nil {
		t.Fatal("legacy workspace identity is no longer absent")
	}
	for _, decorate := range []func(map[string]any){zero.decorateSummary, zero.decorateTimingHeader} {
		document := map[string]any{"scope": "legacy source rosters remain unchanged", "case_profile": map[string]any{"name": "existing"}, "count": 170227}
		before := mustJSON(t, document)
		decorate(document)
		if !bytes.Equal(before, mustJSON(t, document)) {
			t.Fatal("zero workspace profile changed original output bytes")
		}
	}
	want := map[string]any{
		"schema": "gooo/native-workspace-profile/v1", "name": "dependency-closure-v1",
		"basis": "legacy-selected-workspace", "compiler_revision": "205896a413e5dea92366309c35ae85e542d055f6", "go_version": "go1.27.3",
		"selection": "entry import closure over whole already-selected packages; original package/source order",
	}
	if !reflect.DeepEqual(dependencyClosureWorkspaceProfile.identity(), want) {
		t.Fatal("workspace profile lost its fixed distinct identity", dependencyClosureWorkspaceProfile.identity())
	}
	for _, caseProfile := range []nativeCaseProfile{"", uuidCompactCaseProfile, uuidLiteralReportCompactCaseProfile} {
		summary := map[string]any{
			"scope":             "The prior 56 suites, 108,835 cases, source rosters and 1,416 batch boundaries are unchanged.",
			"uuid_parse_scope":  "All prior 56 suites, 108835 cases, 1416 batch boundaries and source rosters remain unchanged.",
			"containment_scope": "Original 57 suites, 130702 case documents and 1741 batch boundaries remain unchanged.",
		}
		header := map[string]any{"boundary_scope": "existing case boundaries"}
		caseProfile.decorateSummary(summary)
		caseProfile.decorateTimingHeader(header)
		oldCaseIdentity := mustJSON(t, summary["case_profile"])
		dependencyClosureWorkspaceProfile.decorateSummary(summary)
		dependencyClosureWorkspaceProfile.decorateTimingHeader(header)
		if !reflect.DeepEqual(summary["workspace_profile"], want) || !reflect.DeepEqual(header["workspace_profile"], want) || !bytes.Equal(oldCaseIdentity, mustJSON(t, summary["case_profile"])) {
			t.Fatal("workspace identity is missing or overwrote independent case profile", caseProfile)
		}
		for _, key := range []string{"scope", "uuid_parse_scope", "containment_scope"} {
			text := fmt.Sprint(summary[key])
			if strings.Contains(text, "source rosters and") || strings.Contains(text, "source rosters remain unchanged") || strings.Contains(text, "source rosters are retained") {
				t.Fatal("opted-in narrowed workspace repeats a false old preservation claim", caseProfile, key, text)
			}
		}
	}
}

func workspaceProfileBatchBytes(t *testing.T, s suite, start int, prepared nativePreparedCaseBatch) []byte {
	t.Helper()
	if prepared.End <= start || prepared.End > len(s.Cases) {
		t.Fatal("invalid original case interval", start, prepared.End)
	}
	if prepared.Raw != nil {
		return prepared.Raw
	}
	batch := s
	batch.Cases = s.Cases[start:prepared.End]
	raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func TestWorkspaceProfileFixed70ClosuresPreserveEveryOriginalCaseAndPartition(t *testing.T) {
	// This is the only full-factory construction in the new tests. No compiler,
	// native command, retained output or Gooo result supplies expected values.
	suites := allNativeSuites(t)
	decimal, err := decimalPayloadCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	suites = append(suites, decimal.Suites...)
	paths, err := pathAliasCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	suites = append(suites, paths...)
	if len(suites) != len(workspaceProfileReviewed70) || len(suites) != 70 {
		t.Fatal("full70 factory roster changed", len(suites))
	}
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	beforeCatalog := contentDigest(mustJSON(t, frozen))
	counts := map[nativeCaseProfile]int{}
	changed, totalCases, totalFiles := 0, 0, 0
	preparedPolicies, prepareCalls, prettyWrites := 0, 0, 0
	for index, s := range suites {
		expected := workspaceProfileReviewed70[index]
		entry := s.Package + ":" + s.Activity
		if entry != expected.entry || len(s.Cases) != expected.cases {
			t.Fatal("original ordered entry/case population changed", index, entry, len(s.Cases))
		}
		beforeCases := contentDigest(mustJSON(t, s.Cases))
		legacy, err := frozen.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		zero, err := nativeWorkspaceProfile("").workspaceForSuite(frozen, s)
		if err != nil || !reflect.DeepEqual(zero, legacy) || contentDigest(mustJSON(t, legacy.Manifest)) != expected.defaultManifest {
			t.Fatal("default workspace changed its exact old manifest/order/bytes/identities", entry, err)
		}
		narrow, err := dependencyClosureWorkspaceProfile.workspaceForSuite(frozen, s)
		if err != nil {
			t.Fatal(entry, err)
		}
		if contentDigest(mustJSON(t, narrow.Manifest)) != expected.closureManifest || strings.Join(narrow.Names, "|") != expected.orderedNames {
			t.Fatal("new closure differs from independent fixed inventory", entry, narrow.Manifest, narrow.Names)
		}
		different := !reflect.DeepEqual(legacy.Manifest, narrow.Manifest)
		if different != expected.changed {
			t.Fatal("changed/isolated workspace classification differs", entry)
		}
		if different {
			changed++
		} else if !reflect.DeepEqual(legacy, narrow) {
			t.Fatal("isolated workspace changed even though its manifest did not", entry)
		}
		for _, name := range narrow.Names {
			if !bytes.Equal(narrow.Sources[name], legacy.Sources[name]) || narrow.Identities[name] != legacy.Identities[name] {
				t.Fatal("profile changed a retained original source or its identity", entry, name)
			}
		}
		// prepare reads only the complete policy value, suite and start. Prove
		// both workspace choices supply the same value before sharing its pure
		// JSON work. Keep each exact interval, nil-vs-raw writer and bytes once
		// per distinct policy for this entry, including all three wire profiles.
		type preparedCaseBatch struct {
			start    int
			prepared nativePreparedCaseBatch
			raw      []byte
			digest   string
		}
		preparedByPolicy := map[nativeCaseBatchPolicy][]preparedCaseBatch{}
		for _, profile := range []nativeCaseProfile{"", uuidCompactCaseProfile, uuidLiteralReportCompactCaseProfile} {
			oldPolicy, err := profile.policy(s, legacy, workspaceProfilePinnedLock())
			if err != nil {
				t.Fatal(entry, profile, err)
			}
			newPolicy, err := profile.policy(s, narrow, workspaceProfilePinnedLock())
			if err != nil {
				t.Fatal(entry, profile, err)
			}
			// The struct is deliberately the whole comparable key, not selected
			// flags: future source-dependent policy drift must fail before reuse.
			if oldPolicy != newPolicy {
				t.Fatal("workspace selection changed the complete case writer/bounds policy", entry, profile, oldPolicy, newPolicy)
			}
			preparedBatches, found := preparedByPolicy[oldPolicy]
			if !found {
				preparedPolicies++
				for start := 0; start < len(s.Cases); {
					prepareCalls++
					prepared, err := oldPolicy.prepare(s, start)
					if err != nil {
						t.Fatal(entry, profile, start, err)
					}
					if prepared.Raw == nil {
						prettyWrites++
					}
					raw := workspaceProfileBatchBytes(t, s, start, prepared)
					preparedBatches = append(preparedBatches, preparedCaseBatch{start, prepared, raw, contentDigest(raw)})
					start = prepared.End
				}
				preparedByPolicy[oldPolicy] = preparedBatches
			}
			start := 0
			for _, batch := range preparedBatches {
				if batch.start != start || batch.prepared.End <= start || batch.prepared.End > len(s.Cases) || len(batch.raw) == 0 || batch.raw[len(batch.raw)-1] != '\n' || contentDigest(batch.raw) != batch.digest {
					t.Fatal("shared original raw case bytes or exact contiguous bounds changed", entry, profile, start)
				}
				if batch.prepared.Raw != nil && !bytes.Equal(batch.prepared.Raw, batch.raw) {
					t.Fatal("prepared bytes changed the original raw writer", entry, profile, start)
				}
				start = batch.prepared.End
			}
			if start != len(s.Cases) || contentDigest(mustJSON(t, s.Cases)) != beforeCases {
				t.Fatal("shared policy skipped or mutated original ordered cases", entry, profile, start)
			}
			batches := len(preparedBatches)
			if profile == uuidLiteralReportCompactCaseProfile && batches != expected.currentBatches {
				t.Fatal("current2153 partition changed", entry, batches, expected.currentBatches)
			}
			counts[profile] += batches
		}
		if contentDigest(mustJSON(t, s.Cases)) != beforeCases {
			t.Fatal("selection/writer mutated original cases", entry)
		}
		totalCases += len(s.Cases)
		totalFiles += len(narrow.Names)
	}
	if changed != 56 || len(suites)-changed != 14 || totalCases != 170227 || totalFiles != 180 || counts[""] != 2427 || counts[uuidCompactCaseProfile] != 2199 || counts[uuidLiteralReportCompactCaseProfile] != 2153 {
		t.Fatal("reviewed finite totals changed", changed, totalCases, totalFiles, counts)
	}
	if preparedPolicies != 74 || prepareCalls != 3293 || prettyWrites != 2337 {
		t.Fatal("distinct original policy work changed", preparedPolicies, prepareCalls, prettyWrites)
	}
	t.Logf("shared exact policy plans: %d; prepare calls: %d; original pretty writer calls: %d", preparedPolicies, prepareCalls, prettyWrites)
	if contentDigest(mustJSON(t, frozen)) != beforeCatalog {
		t.Fatal("profile mutated shared frozen catalog")
	}
}

// This tiny source inventory exercises only package graph selection. Its entry
// keeps the independent StatusText contract path; no declaration/native claim
// is made for these intentionally minimal source texts.
func workspaceProfileGraphFixture(t *testing.T) frozenVerificationCatalog {
	t.Helper()
	packages := []sourcePackage{
		{Path: "go/time", Sources: []string{"ports/time/duration.gooo"}},
		{Path: "go/net/http", Sources: []string{"ports/net/http.gooo"}},
		{Path: "go/math/bits", Sources: []string{"ports/math/bits.gooo"}},
		{Path: "go/image/color", Sources: []string{"ports/image/color.gooo", "ports/image/helper.gooo"}},
		{Path: "go/unicode/utf8", Sources: []string{"ports/unicode/utf8.gooo"}},
	}
	sources := map[string][]byte{
		"ports/time/duration.gooo": []byte("package time\nimport color \"go/image/color\" // shared dependency\n"),
		"ports/net/http.gooo":      []byte("package http\nimport clock \"go/time\"\nimport bits \"go/math/bits\"\n"),
		"ports/math/bits.gooo":     []byte("package bits\nimport \"go/image/color\"\n"),
		"ports/image/color.gooo":   []byte("package color\n"),
		"ports/image/helper.gooo":  []byte("package color\n// Whole-package helper must remain although its name is never imported.\n"),
		"ports/unicode/utf8.gooo":  []byte("package utf8\n"),
	}
	frozen := frozenVerificationCatalog{Catalog: verificationCatalog{Schema: verificationCatalogSchema, Packages: packages}, Sources: sources, Hashes: map[string]string{}}
	for name, raw := range sources {
		frozen.Hashes[name] = strings.TrimPrefix(contentDigest(raw), "sha256:")
	}
	frozen.Raw = mustJSON(t, frozen.Catalog)
	return frozen
}

func workspaceProfileCloneCatalog(t *testing.T, f frozenVerificationCatalog) frozenVerificationCatalog {
	t.Helper()
	copy := frozenVerificationCatalog{Raw: bytes.Clone(f.Raw), Sources: map[string][]byte{}, Hashes: map[string]string{}}
	if err := json.Unmarshal(mustJSON(t, f.Catalog), &copy.Catalog); err != nil {
		t.Fatal(err)
	}
	for name, raw := range f.Sources {
		copy.Sources[name] = bytes.Clone(raw)
	}
	for name, hash := range f.Hashes {
		copy.Hashes[name] = hash
	}
	return copy
}

func workspaceProfileReplaceSource(f *frozenVerificationCatalog, name, source string) {
	f.Sources[name] = []byte(source)
	f.Hashes[name] = strings.TrimPrefix(contentDigest(f.Sources[name]), "sha256:")
}

func TestWorkspaceProfileTransitiveDiamondKeepsWholePackagesAndOriginalOrder(t *testing.T) {
	frozen := workspaceProfileGraphFixture(t)
	s := suite{Package: "go/net/http", Activity: "StatusText"}
	before := contentDigest(mustJSON(t, frozen))
	selected, err := dependencyClosureWorkspaceProfile.workspaceForSuite(frozen, s)
	if err != nil {
		t.Fatal(err)
	}
	// The shared color node appears once after both importers according to the
	// original manifest, not DFS order. Both whole-package source files remain.
	want := frozen.Catalog.Packages[:4]
	if !reflect.DeepEqual(selected.Manifest.Packages, want) || len(selected.Names) != 5 || selected.Names[4] != "ports/image/helper.gooo" {
		t.Fatal("transitive/diamond selection reordered, split or duplicated packages", selected.Manifest.Packages, selected.Names)
	}
	if contentDigest(mustJSON(t, frozen)) != before {
		t.Fatal("selection mutated shared catalog")
	}
	selected.Sources["ports/net/http.gooo"][0] ^= 1
	selected.Manifest.Packages[0].Sources[0] = "mutated.gooo"
	if contentDigest(mustJSON(t, frozen)) != before {
		t.Fatal("selected source or manifest aliases frozen storage")
	}

	// A future hash-bound file genuinely added to an already-selected package
	// remains whole; selector does not freeze package file counts in code.
	future := workspaceProfileCloneCatalog(t, frozen)
	future.Catalog.Packages[3].Sources = append(future.Catalog.Packages[3].Sources, "ports/image/future-helper.gooo")
	workspaceProfileReplaceSource(&future, "ports/image/future-helper.gooo", "package color\n")
	got, err := dependencyClosureWorkspaceProfile.workspaceForSuite(future, s)
	if err != nil || len(got.Names) != 6 || got.Names[5] != "ports/image/future-helper.gooo" {
		t.Fatal("whole-package rule silently dropped a future bound helper", got.Names, err)
	}
}

func TestWorkspaceProfileRejectsBrokenSourceGraphAndUnsupportedImports(t *testing.T) {
	base := workspaceProfileGraphFixture(t)
	s := suite{Package: "go/net/http", Activity: "StatusText"}
	mutations := map[string]func(*frozenVerificationCatalog){
		"missing imported package": func(f *frozenVerificationCatalog) {
			f.Catalog.Packages = append(f.Catalog.Packages[:3], f.Catalog.Packages[4:]...)
		},
		"missing helper bytes":              func(f *frozenVerificationCatalog) { delete(f.Sources, "ports/image/helper.gooo") },
		"missing helper hash":               func(f *frozenVerificationCatalog) { delete(f.Hashes, "ports/image/helper.gooo") },
		"helper bytes changed without hash": func(f *frozenVerificationCatalog) { f.Sources["ports/image/helper.gooo"][0] ^= 1 },
		"duplicate package": func(f *frozenVerificationCatalog) {
			f.Catalog.Packages = append(f.Catalog.Packages, f.Catalog.Packages[1])
		},
		"duplicate source in package": func(f *frozenVerificationCatalog) {
			f.Catalog.Packages[3].Sources = append(f.Catalog.Packages[3].Sources, "ports/image/color.gooo")
		},
		"source bound to two packages": func(f *frozenVerificationCatalog) {
			f.Catalog.Packages[2].Sources = append(f.Catalog.Packages[2].Sources, "ports/image/color.gooo")
		},
		"self cycle": func(f *frozenVerificationCatalog) {
			workspaceProfileReplaceSource(f, "ports/image/color.gooo", "package color\nimport \"go/image/color\"\n")
		},
		"transitive cycle": func(f *frozenVerificationCatalog) {
			workspaceProfileReplaceSource(f, "ports/image/color.gooo", "package color\nimport \"go/net/http\"\n")
		},
		"unreachable self cycle": func(f *frozenVerificationCatalog) {
			workspaceProfileReplaceSource(f, "ports/unicode/utf8.gooo", "package utf8\nimport \"go/unicode/utf8\"\n")
		},
		"unreachable absent import": func(f *frozenVerificationCatalog) {
			workspaceProfileReplaceSource(f, "ports/unicode/utf8.gooo", "package utf8\nimport \"go/absent\"\n")
		},
	}
	for _, line := range []string{"import (", "import 'go/time'", "import clock", "import \"go/time\";", "import \"go/time\" extra", "import clock = \"go/time\""} {
		line := line
		mutations["unsupported "+line] = func(f *frozenVerificationCatalog) {
			workspaceProfileReplaceSource(f, "ports/net/http.gooo", "package http\n"+line+"\n")
		}
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			frozen := workspaceProfileCloneCatalog(t, base)
			mutate(&frozen)
			if _, err := dependencyClosureWorkspaceProfile.workspaceForSuite(frozen, s); err == nil {
				t.Fatal("broken graph/source was accepted")
			}
		})
	}
	if _, err := nativeWorkspaceProfile("other").workspaceForSuite(base, s); err == nil {
		t.Fatal("unknown direct profile selected sources")
	}
	if _, err := dependencyClosureWorkspaceProfile.workspaceForSuite(base, suite{Package: "go/net/http", Activity: "Unregistered"}); err == nil {
		t.Fatal("unknown entry selected sources")
	}
}

func workspaceProfileImage(t *testing.T, s suite, selected suiteWorkspace) workspaceImage {
	t.Helper()
	contract, err := contractFor(s)
	if err != nil {
		t.Fatal(err)
	}
	image := workspaceImage{}
	for _, pkg := range selected.Manifest.Packages {
		p := workspacePackage{Path: pkg.Path}
		for _, name := range pkg.Sources {
			p.Sources = append(p.Sources, workspaceSource{Filename: name, Digest: selected.Identities[name].Digest})
		}
		if pkg.Path == s.Package {
			p.Exports = []workspaceExport{{Name: s.Activity, Kind: "activity", ID: contract.ID, InputTypes: contract.InputEntities, OutputType: whitespaceResultContract(s).Entity}}
		}
		image.Packages = append(image.Packages, p)
	}
	return image
}

// Source-bound synthetic receipts retain real inspector/replay checks. This
// helper never executes a compiler/generated program and makes no native claim.
func workspaceProfileNativeFixture(t *testing.T, profile nativeWorkspaceProfile) ([]byte, suite, expectedIdentity, suiteWorkspace, frozenVerificationCatalog) {
	t.Helper()
	s := suite{Package: "app/color-inspector", Activity: "InspectRGB", Cases: []testCase{{Inputs: []any{int64(12), int64(34), int64(56)}, Expected: inspectRGBOracle([]uint8{12, 34, 56})}}}
	raw, one, identity := contractFixture(t, s)
	frozen, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	selected, err := profile.workspaceForSuite(frozen, one)
	if err != nil {
		t.Fatal(err)
	}
	var receipt envelope
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	identity.CompilerRevision = workspaceProfilePinnedLock().CompilerRevision
	identity.Sources, identity.SourceBytes = selected.Identities, selected.Sources
	receipt.Result.Runtime.Producer = identity.CompilerRevision
	receipt.Result.Program.Image = workspaceProfileImage(t, one, selected)
	raw = mustJSON(t, receipt)
	if _, err := inspect(raw, one, false, identity); err != nil {
		t.Fatal("invalid pinned source-bound synthetic fixture", err)
	}
	return raw, repeatNativeCase(one, 65), identity, selected, frozen
}

func workspaceProfileTimingConfig(t *testing.T, root string, s suite, profile nativeWorkspaceProfile, frozen frozenVerificationCatalog) nativeTimingConfig {
	t.Helper()
	cfg := nativeTimingTestConfig(t, root, []suite{s})
	cfg.WorkspaceProfile, cfg.Lock = profile, workspaceProfilePinnedLock()
	cfg.Catalog = frozen
	write := func(name string, raw []byte) {
		t.Helper()
		path := filepath.Join(cfg.RepositoryRoot, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("upstream.json", mustJSON(t, cfg.Lock))
	write("verification-catalog.json", frozen.Raw)
	for name, raw := range frozen.Sources {
		write(name, raw)
	}
	cfg.CompilerIdentity = mustJSON(t, map[string]any{"version": cfg.Lock.CompilerVersion, "compiler_source_sha": cfg.Lock.CompilerRevision, "go_version": cfg.Lock.GoVersion, "native_go_required": cfg.Lock.GoVersion, "vcs_modified": "false"})
	return cfg
}

func workspaceProfileRecorder(t *testing.T, root string, s suite, profile nativeWorkspaceProfile, frozen frozenVerificationCatalog) (*nativeTiming, string) {
	t.Helper()
	cfg := workspaceProfileTimingConfig(t, root, s, profile, frozen)
	recorder, err := newNativeTimingWithSnapshot(cfg, nativeTimingTestSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recorder.close(fmt.Errorf("synthetic workspace profile cleanup"), nil) })
	return recorder, cfg.Output
}

func workspaceProfileAssertTimingInputs(t *testing.T, header map[string]any) {
	t.Helper()
	inputs := header["identity"].(map[string]any)["source_files"].(map[string]any)
	expected, err := nativeExpectedVerificationContract()
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != len(expected.Sources) {
		t.Fatal("timing provenance source count changed", len(inputs))
	}
	for _, name := range []string{"cmd/verify/workspace_profile.go", "cmd/verify/workspace_profile_test.go", lastIndexSource, lastIndexConsumerSource, "tools/check-last-index-inputs.py", "cmd/build-demo/api_policy_build.go"} {
		if inputs[name] == nil {
			t.Fatal("timing provenance omitted a required profile or builder source", name)
		}
	}
}

func TestWorkspaceProfileSyntheticNativeReplayAndExactTimingMap(t *testing.T) {
	for _, profile := range []nativeWorkspaceProfile{"", dependencyClosureWorkspaceProfile} {
		name := string(profile)
		if name == "" {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			raw, s, _, selected, frozen := workspaceProfileNativeFixture(t, profile)
			root := timingIntegrationRoot(t)
			recorder, path := workspaceProfileRecorder(t, root, s, profile, frozen)
			planned := caseProfileIntegrationPlan(t, "", s, selected, workspaceProfilePinnedLock())
			fake := fakeNativeCommand{templates: map[string][]byte{suiteEvidenceDirectory(root, s): raw}, calls: map[string][]string{}}
			ctx := workerTestContext(t)
			verifier := newNativeSuiteVerifier(ctx, "unused", root, workspaceProfilePinnedLock(), nil)
			verifier.timing, verifier.execute = recorder, caseProfileIntegrationCommand(t, root, s, planned, &fake)
			span := recorder.beginSuite(0, s)
			result, err := verifier.verify(ctx, s, selected)
			span.finish(result, err, ctx.Err())
			if err != nil || result.Observation.Passed != 65 || result.Observation.Batches != 3 {
				t.Fatal("profile lost actual inspected synthetic coverage", result, err)
			}
			if len(fake.calls) != 3 {
				t.Fatal("missing or additional native batch", fake.calls)
			}
			for _, calls := range fake.calls {
				if !reflect.DeepEqual(calls, []string{"execute", "replay"}) {
					t.Fatal("saved replay omitted", calls)
				}
			}
			if err := recorder.close(nil, nil); err != nil {
				t.Fatal(err)
			}
			rows := timingIntegrationRows(t, path)
			header, footer := rows[0], rows[len(rows)-1]
			if footer["status"] != "COMPLETE" || footer["verified_cases"] != float64(65) || footer["verified_batches"] != float64(3) {
				t.Fatal("timing footer lost verified finite coverage", footer)
			}
			if _, present := header["case_profile"]; present {
				t.Fatal("workspace profile introduced a case profile")
			}
			if profile == "" {
				if _, present := header["workspace_profile"]; present {
					t.Fatal("default timing gained workspace metadata")
				}
			} else if !equalValue(mustJSON(t, header["workspace_profile"]), profile.identity()) {
				t.Fatal("explicit timing identity missing")
			}
			workspaceProfileAssertTimingInputs(t, header)
			mapSHA := contentDigest(mustJSON(t, selected.Identities))
			for _, row := range rows {
				if row["kind"] == "suite_end" && (!equalValue(mustJSON(t, row["source_set"]), selected.Identities) || row["source_set_sha256"] != mapSHA) {
					t.Fatal("timing lost exact selected map", row)
				}
				if row["kind"] == "batch_end" && row["source_set_sha256"] != mapSHA {
					t.Fatal("batch does not bind same selected map", row)
				}
			}
			for index := range planned {
				dir := filepath.Join(suiteEvidenceDirectory(root, s), fmt.Sprintf("%03d", index))
				original, err := os.ReadFile(filepath.Join(dir, "execution.json"))
				if err != nil {
					t.Fatal(err)
				}
				var replay envelope
				if err := readJSON(filepath.Join(dir, "replay.json"), &replay); err != nil {
					t.Fatal(err)
				}
				if replay.ReplayedFrom != contentDigest(original) {
					t.Fatal("actual saved replay lost raw outer parent")
				}
			}
		})
	}
}

func TestWorkspaceProfileSyntheticNativeRefusesSourceImageAndSavedParentTampering(t *testing.T) {
	for _, mode := range []string{"broad image with narrow manifest", "missing whole-package helper", "wrong saved parent"} {
		t.Run(mode, func(t *testing.T) {
			raw, s, _, selected, frozen := workspaceProfileNativeFixture(t, dependencyClosureWorkspaceProfile)
			root := timingIntegrationRoot(t)
			recorder, path := workspaceProfileRecorder(t, root, s, dependencyClosureWorkspaceProfile, frozen)
			fake := fakeNativeCommand{templates: map[string][]byte{suiteEvidenceDirectory(root, s): raw}, calls: map[string][]string{}}
			mutations := 0
			fake.mutate = func(action string, receipt map[string]any) {
				if mode == "wrong saved parent" {
					if action == "replay" {
						receipt["replayed_from_sha256"] = "sha256:" + strings.Repeat("0", 64)
						mutations++
					}
					return
				}
				if action != "execute" {
					return
				}
				image := workspaceProfileImage(t, s, selected)
				if mode == "broad image with narrow manifest" {
					broad, err := frozen.workspaceForSuite(s)
					if err != nil {
						t.Fatal(err)
					}
					image = workspaceProfileImage(t, s, broad)
				} else {
					image.Packages[0].Sources = image.Packages[0].Sources[:len(image.Packages[0].Sources)-1]
				}
				receipt["result"].(map[string]any)["program"].(map[string]any)["workspace_image"] = image
				mutations++
			}
			ctx := workerTestContext(t)
			verifier := newNativeSuiteVerifier(ctx, "unused", root, workspaceProfilePinnedLock(), nil)
			verifier.timing, verifier.execute = recorder, fake.execute
			span := recorder.beginSuite(0, s)
			result, originalErr := verifier.verify(ctx, s, selected)
			span.finish(result, originalErr, ctx.Err())
			if originalErr == nil || result.Observation.Passed != 0 || mutations != 1 {
				t.Fatal("tampered profile became a verified observation", result, originalErr, mutations)
			}
			wanted := "workspace"
			if mode == "wrong saved parent" {
				wanted = "saved replay is not bound"
			}
			if !strings.Contains(originalErr.Error(), wanted) {
				t.Fatal("tampering refused for unrelated reason", originalErr)
			}
			if err := recorder.close(originalErr, nil); err == nil {
				t.Fatal("invalid receipt completed timing")
			}
			rows := timingIntegrationRows(t, path)
			if rows[len(rows)-1]["status"] == "COMPLETE" || rows[len(rows)-1]["verified_batches"] != float64(0) {
				t.Fatal("invalid receipt yielded complete timing coverage")
			}
			wantCalls := []string{"execute"}
			if mode == "wrong saved parent" {
				wantCalls = append(wantCalls, "replay")
			}
			if len(fake.calls) != 1 || !reflect.DeepEqual(fake.calls[filepath.Join(suiteEvidenceDirectory(root, s), "000")], wantCalls) {
				t.Fatal("verification continued after failed evidence", fake.calls)
			}
		})
	}
}

func TestWorkspaceProfileTimingRejectsCatalogSubsetBroadMapAndLaterDrift(t *testing.T) {
	for _, mode := range []string{"omitted helper", "broad catalog members", "later map drift"} {
		t.Run(mode, func(t *testing.T) {
			_, s, _, selected, frozen := workspaceProfileNativeFixture(t, dependencyClosureWorkspaceProfile)
			root := timingIntegrationRoot(t)
			recorder, path := workspaceProfileRecorder(t, root, s, dependencyClosureWorkspaceProfile, frozen)
			span := recorder.beginSuite(0, s)
			identities := func() map[string]sourceIdentity {
				copy := map[string]sourceIdentity{}
				for name, identity := range selected.Identities {
					copy[name] = identity
				}
				return copy
			}
			for batch, bounds := range [][2]int{{0, 32}, {32, 65}} {
				if mode != "later map drift" && batch == 1 {
					break
				}
				bt := recorder.beginBatch(s, batch, bounds[0], bounds[1])
				if bt == nil {
					t.Fatal("valid batch did not start")
				}
				for _, phase := range nativeTimingPhaseNames[1:6] {
					bt.phase(phase)
				}
				sources := identities()
				if mode == "omitted helper" || mode == "later map drift" && batch == 1 {
					delete(sources, "ports/image/ycbcr.gooo")
				}
				if mode == "broad catalog members" {
					broad, err := frozen.workspaceForSuite(s)
					if err != nil {
						t.Fatal(err)
					}
					sources = broad.Identities
				}
				bt.bind(expectedIdentity{CompilerRevision: workspaceProfilePinnedLock().CompilerRevision, GoVersion: "go1.27.3", ManifestDigest: contentDigest([]byte("selected manifest")), CasesDigest: contentDigest([]byte("original cases")), Sources: sources}, contentDigest([]byte("raw execution receipt")))
				bt.finish(nil, nil, true)
			}
			span.finish(nativeTimingTestObservation(s, 2), nil, nil)
			if err := recorder.close(nil, nil); err == nil || !strings.Contains(err.Error(), "exact selected closure") {
				t.Fatal("recorder accepted wrong first/later per-entry map", mode, err)
			}
			rows := timingIntegrationRows(t, path)
			if rows[len(rows)-1]["status"] == "COMPLETE" {
				t.Fatal("map mismatch became complete coverage")
			}
		})
	}
}

func TestWorkspaceProfileTimingRejectsBadPinsGraphAndMissingInputBeforeOutput(t *testing.T) {
	for _, mode := range []string{"unknown profile", "wrong compiler version", "missing selected helper", "changed helper original hash", "missing profile implementation input", "missing profile test input"} {
		t.Run(mode, func(t *testing.T) {
			_, s, _, _, frozen := workspaceProfileNativeFixture(t, dependencyClosureWorkspaceProfile)
			cfg := workspaceProfileTimingConfig(t, timingIntegrationRoot(t), s, dependencyClosureWorkspaceProfile, workspaceProfileCloneCatalog(t, frozen))
			switch mode {
			case "unknown profile":
				cfg.WorkspaceProfile = nativeWorkspaceProfile("unknown")
			case "wrong compiler version":
				cfg.Lock.CompilerVersion = "0.6.34-dev"
			case "missing selected helper":
				delete(cfg.Catalog.Sources, "ports/image/ycbcr.gooo")
			case "changed helper original hash":
				cfg.Catalog.Sources["ports/image/ycbcr.gooo"][0] ^= 1
			case "missing profile implementation input":
				if err := os.Remove(filepath.Join(cfg.RepositoryRoot, "cmd/verify/workspace_profile.go")); err != nil {
					t.Fatal(err)
				}
			case "missing profile test input":
				if err := os.Remove(filepath.Join(cfg.RepositoryRoot, "cmd/verify/workspace_profile_test.go")); err != nil {
					t.Fatal(err)
				}
			}
			recorder, err := newNativeTimingWithSnapshot(cfg, nativeTimingTestSnapshot)
			if err == nil {
				_ = recorder.close(fmt.Errorf("unexpected accepted timing fixture"), nil)
				t.Fatal("invalid profile provenance opened timing output", mode)
			}
			if _, err := os.Lstat(cfg.Output); !os.IsNotExist(err) {
				t.Fatal("invalid profile created or replaced output", mode, err)
			}
		})
	}
}
