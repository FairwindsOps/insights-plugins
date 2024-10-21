package image

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestFilterAndSort(t *testing.T) {
	tags := []string{"6b6d653", "v0.0.14", "v0.0.22", "v0.0.16", "v0.0.16-alpine", "v0.0.14-beta2"}
	newestTags, err := filterAndSort(tags, "v0.0.14-beta1")
	assert.NoError(t, err)
	assert.Equal(t, []string{"v0.0.14", "v0.0.16", "v0.0.22"}, newestTags)
}

func TestFilterAndSor1(t *testing.T) {
	tags := []string{"6b6d653", "v0.0.14", "v0.0.14-beta1", "v0.0.22", "v0.0.22-alpine", "v0.0.16"}
	newestTags, err := filterAndSort(tags, "v0.0.14")
	assert.NoError(t, err)
	assert.Equal(t, []string{"v0.0.16", "v0.0.22"}, newestTags)
}

func TestFilterAndSort2(t *testing.T) {
	tags := []string{"6b6d653", "v0.0.14", "v0.0.14-beta1", "v0.0.22", "v0.0.16"}
	newestTags, err := filterAndSort(tags, "6b6d653")
	assert.NoError(t, err)
	assert.Equal(t, []string{}, newestTags)
}

func TestFilterAndSort3(t *testing.T) {
	tags := []string{"0.1.2-ubuntu", "0.1.3-alpine", "0.1.1-alpine", "0.1.1-beta1"}
	newestTags, err := filterAndSort(tags, "0.1.0-alpine")
	assert.NoError(t, err)
	assert.Equal(t, []string{"0.1.1-alpine", "0.1.3-alpine"}, newestTags)
}

func TestFilterAndSort4(t *testing.T) {
	tags := []string{"1.13.9", "1.14", "1.14.5", "1.13.2"}
	newestTags, err := filterAndSort(tags, "1.13.2")
	assert.NoError(t, err)
	assert.Equal(t, []string{"1.13.9", "1.14", "1.14.5"}, newestTags)
}

func TestFilterAndSort5(t *testing.T) {
	tags := []string{"5e3c1a8", "d1c998a", "98f5fc0", "v0.7.0-beta.0", "fc78dce", "3156caf", "6875701", "104d63f", "7dd3f4e", "b86acb0", "27e686b", "3b2d270", "7875368", "aeef7dc", "7df6ce9", "193c5e2", "v0.7.0", "c08984a", "cbfd4fc", "b5a38b9",
		"05517af", "3907629", "2d69622", "ee79b6f", "b3410cb", "66419af", "5afa0e4", "74d27aa", "4f67c80", "aeb0fdb", "da73c81", "5bc0bae", "9757b2a", "6261b48", "1fcb002e", "efca96d5", "3ca100fd", "3a747cda", "a99821a1", "2f99b315", "aca339d6",
		"c6ebaa7e", "e1571326", "49f91f9f", "dca819df", "ff19e63e", "8b0a39a5", "6a39a868", "076ecb4e", "v0.7.1", "113c424c", "f3910e0d", "bf50c5d0", "8bef7126", "7c552a5d", "546d3f1d", "14f37da9", "2235be30", "6523015b", "6147e891", "57075123",
		"4b4742d7", "4c31b977", "7528b977", "4f6d7717", "d2ca1ba2", "331df1b3", "a3921f55", "667e98b8", "e0474fb2", "v0.8.0-alpha.0", "v0.7.2", "36ffc071", "fe58b946", "dfc7549d", "d9cf033b", "459bb84e", "7544dea9", "2e95421d", "01d706e0", "v0.8.0-beta.0",
		"2253cbe7", "98f67a88", "a8482eae", "755f4ac4", "f4a16cb1", "111883a2", "8d107237", "fbf0e5e0", "7852fd05", "3201d126", "5844f255", "5e17e1b2", "b65aadc2", "5a213cc5", "1690b439", "v0.8.0", "be0cd5c7", "52ce9593", "56927881", "57e52689", "5908d2f1",
		"ecec081a", "80e97c98", "edd22250", "1c10f340", "bb0f4796", "72813494", "fe00f40a", "1205dc5e", "4c199c90", "92154dbc", "f3a8a736", "57081bac", "fce7bdd3", "0a7a1818", "9c6d81a9", "93faf9c1", "a0e4e46b", "a0e1b864", "9c16efdb", "48cd58b2", "3c265d11",
		"07c34114", "043a88cd", "c475eb63", "0c5f4fec", "fd32827b", "31b873ac", "4f35c564", "f3bc4fad", "84770edf", "8fa533c3", "v0.8.1", "55fadf10", "6a909708", "b5ba6d6f", "b7f3d2c8", "8852b480", "a4173c80", "2711fabe", "16fd5b42", "0ec98cfa", "fa79f46b",
		"07fbbc1a", "d3446a83", "04c875c7", "ae50b79f", "be95598d", "1a014dde", "9157b49d", "8e54b32d", "30d6664d", "7cead3d9", "bd08bf6a", "70bc3e84", "dd1f8d4d", "e2424c66", "f7c7c998", "870050df", "55679c6b", "391737fe", "be9b4828", "9d418eae", "caa130b7",
		"c6cd522a", "458c7193", "1b9b83a4", "a14cd359", "57d77289", "v0.9.0-alpha.0", "0bf85e72", "5d1db973", "b76c3f74", "592b8f98", "13266f5c", "13ebd873", "007bc1f8", "a9751fce", "e43a3976", "0b5f963b", "1ae7586c", "65138f5e", "873f4835", "8d611455",
		"25048af6", "9aa1ce66", "a67c23ed", "91bcfa33", "f77ab02c", "d9b1b9e4", "v0.9.0-beta.0", "ab4d15ef", "9299d14e", "8ca84665", "590b04d7", "9a6510ea", "9772c56d", "2e03be48", "e3b1fb7e", "db1a11ba", "8c983150", "v0.9.0", "5d6f92cc", "4dc46d68",
		"2d1c2137", "ab03b44b", "0050432f", "222d46ab", "7975c924", "146c4c01", "8fa48c21", "c7cdf6f4", "5f8c1aca", "892eafbf", "946b1f27", "465541a6", "c67c53c0", "7575e263", "8941df04", "36a888a0", "36c8cff0", "84ceeb74", "03e742b2", "23541ed0",
		"v0.9.1", "95e8b7de", "0c569472", "a51e66a1", "20bf2ecf", "f7f0e9f1", "4f622c74", "9920fae4", "40280006", "d4a675ee", "3716902b8", "dc670926e", "v0.10.0-alpha.0", "c7ed46e12", "8c99af7bf", "dcba8ebd0", "582371a1d", "aa7071b4e", "57ab30c7b",
		"a54e76679", "e476d17fc", "c9583268a", "3eee46712", "v0.10.0", "f1d591a53", "c6d93a809", "ef6dde2ca", "1a0e0217e", "62258bffa", "d4b0e82c4", "d2cedd50e", "96e82e218", "c432b8d76", "87fbfebe5", "ab5ddbfbd", "49d40226b", "3a3c84006", "1977a9970",
		"e627c4c18", "4ff262081", "716b11d8d", "abb680756", "b8bd8dc66", "6b169875c", "d2b12fb15", "9dd6b1073", "816bbf54d", "82701029a", "29d195309", "3f461ee17", "18106e4e9", "9cd00ce3f", "689372b4d", "5d532246d", "v0.10.1", "4aa10bedc", "c7e5a9f17",
		"608111629", "b502d8c21", "b91b7d8d3", "44abb9b26", "97344e6a3", "b6b93ee8d", "2f03013a6", "3fa4a8310", "94eae710a", "9354c9781", "482eac596", "1a12c5d99", "v0.11.0-beta.0", "13fcbb938", "ddabf64e0", "7b0668887", "bd36cb42f", "68df81ea4",
		"0bb2e8d3f", "075dac6e4", "d9043e512", "7f9d6c113", "a573e8a8a", "1c424dd21", "533fc2d62", "d3a418642", "dd0c89aa6", "c0229d2b7", "787d119a7", "2991356c5", "57f6dad0c", "v0.11.0", "b030b7eb4", "d474a5f0b", "4ec682db2", "03e389777", "6b7c51c3d",
		"7b56bca28", "3bd755e5c", "1f6a4c758", "3dd05e900", "082122040", "4d316ea97", "8d12d351ec3aef06dc", "f57ee108c", "7dbdc5edb", "87aedeb04", "v0.12.0-beta.0", "v0.12.0-beta.1", "v0.11.1", "1d0d3e340", "v0.12.0", "v0.13.0-alpha.0", "v0.13.0", "canary",
		"v0.13.1", "v0.14.0-alpha.0", "v0.14.0-alpha.1", "v0.14.0", "v0.14.1", "v0.14.2", "v0.15.0-alpha.0", "v0.15.0-alpha.0-ubi", "v0.15.0-alpha.1", "v0.15.0-alpha.1-ubi", "v0.15.0-alpha.2", "v0.15.0-alpha.2-ubi", "v0.14.3", "v0.15-alpha.3", "v0.15-alpha.3-ubi",
		"v0.15.0-beta.0", "v0.15.0-beta.0-ubi", "v0.15.0-beta.1", "v0.15.0", "v0.15.1", "v0.15.2", "v0.16.0-alpha.0", "v0.16.0-alpha.1", "v0.16.0", "v0.16.1", "v1.0.0-alpha.0", "v1.0.0-alpha.1", "v1.0.0-beta.0", "v1.0.0-beta.1", "v1.0.0", "v1.0.1", "v1.0.2",
		"v1.0.3", "v1.1.0-alpha.0", "v1.1.0-alpha.1", "v1.0.4", "v1.1.0", "v1.2.0-alpha.0", "v1.2.0-alpha.1", "v1.2.0-alpha.2", "v1.2.0", "v1.1.1", "v1.3.0-alpha.0", "v1.3.0-alpha.1", "v1.3.0-beta.0", "v1.3.0", "v1.3.1", "v1.4.0-alpha.0", "v1.4.0-richardw.2",
		"v1.4.0-alpha.1", "v1.4.0-beta.0", "v1.4.0-beta.1", "v1.4.0", "v1.5.0-alpha.0", "v1.4.1", "v1.3.2", "v1.4.2", "v1.5.0-beta.0", "v1.3.3", "v1.4.3", "v1.5.0-beta.1", "v1.5.0", "v1.5.1", "v1.5.2", "v1.4.4", "v1.5.3", "v1.6.0-alpha.0", "v1.5.4",
		"v1.6.0-alpha.1", "999450fd4add69e26ba04d001b811863cba8175b", "v1.6.0-alpha.2", "v1.6.0-beta.0", "v1.6.0", "v1.6.1", "v1.7.0-alpha.0", "v1.7.0-alpha.1", "v1.7.0-beta.0", "v1.7.0", "v1.6.2", "v1.5.5", "v1.7.1", "v1.6.3", "v1.7.2", "v1.8.0-alpha.0",
		"sha256-04397354e817c8f4d682c5ebdbca36f73fa3aa186d5b25ae90bdc56543615f4f.sig", "v1.8.0-alpha.1", "sha256-90922761d46452ee46e9ef304d3760f25160d8f5f271639d6814c93d0bf2c293.sig", "v1.8.0-alpha.2",
		"sha256-a618aecc88dda4dfc3c751d1b107fb28bb1ba0419bcc55325f7c47b378946782.sig", "v1.8.0-beta.0", "sha256-1b07599c9f53b1b34e606a05eae1d4b51e6738b51a0c537c2571183ae32a107d.sig", "v1.8.0", "sha256-e7b6203ccb37f0af23458a4828bd004b5edc8212276c5221408eb8b2a97c938a.sig",
		"v1.8.1", "sha256-2b61dabbff4ef838a9990530ea8bba7a44b88df9464558a84925e7a4b65f0321.sig", "v1.8.2", "sha256-c010246124c26a4ef9503e1b2d415d092051e3fa4fad03681437f1e842a6424f.sig", "v1.7.3", "sha256-77916fc9b98ed41285c50cc9525e186f13fecb38cf1aa98b56694c9c7f729257.sig",
		"v1.9.0-alpha.0", "sha256-81691f5fc0129763020aead07af6275025c94b3869f61c1cc906a31fadf56593.sig", "v1.9.0-beta.0", "sha256-7cdd0725b67c6eedfbb7c3a8f9dfa28027aaa6654871a24e7197eca492ef3277.sig", "v1.9.0-beta.1",
		"sha256-44b45d7700e5ab4a56be5c5186067ca8eb7ad65785d0824e37a9ef51df26bdda.sig", "v1.9.0", "sha256-775ed93db555bbded3efd81ac9de493f0917e70a849658cdc8db93bc601eb589.sig", "v1.9.1", "sha256-df7f0b5186ddb84eccb383ed4b10ec8b8e2a52e0e599ec51f98086af5f4b4938.sig",
		"v1.10.0-alpha.0", "sha256-1b380b626e17e789135c8f0fe582352bdb67998af2df952e3df65388185cbce6.sig", "v1.10.0-beta.0", "sha256-a5b6fdcd7f24480e0caf7e36c8d256a05afc97da421f5ffeaa3f20311ecf71a7.sig", "v1.10.0",
		"sha256-a19dc8e0044e291e8a474e8da20740c4a09770d0011be4cfe001245b9790a87c.sig", "v1.11.0-alpha.0", "sha256-02297c6513a973ab5e85a463a98540a4139d9f24df2bd3ee2d218e2d84b06c5d.sig", "v1.9.2-beta.0",
		"sha256-3d1472d9d7406e4a6816f39dd9f0b4be3302113cdc3a726882e1055bed11727a.sig", "v1.9.2", "sha256-ccd3686f10bfe8aae283cc836d68916f3c74aba9015ca99417e13a9249df5dd4.sig", "v1.10.1", "v1.11.1", "v1.10", "v1.11", "v1.11.0", "sha256-b5657161d2c2f74ab292da4cbaba3923bc74112525f70c3d884fb3c6ef83c303.sig"}
	newestTags, err := filterAndSort(tags, "v1.10.0")
	assert.NoError(t, err)
	assert.Equal(t, []string{"v1.10.1", "v1.11", "v1.11.0", "v1.11.1"}, newestTags)
}

func TestGetNewestVersionsToScan(t *testing.T) {
	t.Skip("This test is intended to only run locally manually")

	var allReports []models.ImageReport
	var allImages []models.Image

	b, err := os.ReadFile("testdata/all-reports-sample.json")
	assert.NoError(t, err)
	err = json.Unmarshal(b, &allReports)
	assert.NoError(t, err)

	b, err = os.ReadFile("testdata/all-images-sample.json")
	assert.NoError(t, err)
	err = json.Unmarshal(b, &allImages)
	assert.NoError(t, err)

	images := GetNewestVersionsToScan(context.Background(), allReports, allImages, map[string]string{})
	assert.NotZero(t, images)
}
