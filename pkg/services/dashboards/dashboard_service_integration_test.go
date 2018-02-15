package dashboards

import (
	"testing"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/services/guardian"
	"github.com/grafana/grafana/pkg/services/sqlstore"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/models"

	. "github.com/smartystreets/goconvey/convey"
)

func TestIntegratedDashboardService(t *testing.T) {
	Convey("Dashboard service integration tests", t, func() {
		sqlstore.InitTestDB(t)
		var testOrgId int64 = 1

		Convey("Given saved folders and dashboards in organization A", func() {
			savedFolder := insertTestFolder("Saved folder", testOrgId)
			savedDashInFolder := insertTestDashboard("Saved dash in folder", testOrgId, savedFolder.Id)
			insertTestDashboard("Other saved dash in folder", testOrgId, savedFolder.Id)
			savedDashInGeneralFolder := insertTestDashboard("Saved dashboard in general folder", testOrgId, 0)
			otherSavedFolder := insertTestFolder("Other saved folder", testOrgId)

			Convey("Should return dashboard model", func() {
				So(savedFolder.Title, ShouldEqual, "Saved folder")
				So(savedFolder.Slug, ShouldEqual, "saved-folder")
				So(savedFolder.Id, ShouldNotEqual, 0)
				So(savedFolder.IsFolder, ShouldBeTrue)
				So(savedFolder.FolderId, ShouldEqual, 0)
				So(len(savedFolder.Uid), ShouldBeGreaterThan, 0)

				So(savedDashInFolder.Title, ShouldEqual, "Saved dash in folder")
				So(savedDashInFolder.Slug, ShouldEqual, "saved-dash-in-folder")
				So(savedDashInFolder.Id, ShouldNotEqual, 0)
				So(savedDashInFolder.IsFolder, ShouldBeFalse)
				So(savedDashInFolder.FolderId, ShouldEqual, savedFolder.Id)
				So(len(savedDashInFolder.Uid), ShouldBeGreaterThan, 0)
			})

			// Basic validation tests

			Convey("When saving a dashboard with non-existing id", func() {
				cmd := models.SaveDashboardCommand{
					OrgId: testOrgId,
					Dashboard: simplejson.NewFromAny(map[string]interface{}{
						"id":    float64(123412321),
						"title": "Expect error",
					}),
				}

				err := callSaveWithError(cmd)

				Convey("It should result in not found error", func() {
					So(err, ShouldNotBeNil)
					So(err, ShouldEqual, models.ErrDashboardNotFound)
				})
			})

			// Given other organization

			Convey("Given organization B", func() {
				var otherOrgId int64 = 2

				Convey("When saving a dashboard with id that are saved in organization A", func() {
					cmd := models.SaveDashboardCommand{
						OrgId: otherOrgId,
						Dashboard: simplejson.NewFromAny(map[string]interface{}{
							"id":    savedDashInFolder.Id,
							"title": "Expect error",
						}),
						Overwrite: false,
					}

					err := callSaveWithError(cmd)

					Convey("It should result in not found error", func() {
						So(err, ShouldNotBeNil)
						So(err, ShouldEqual, models.ErrDashboardNotFound)
					})
				})

				permissionScenario("Given user has permission to save", true, func(sc *dashboardPermissionScenarioContext) {
					Convey("When saving a dashboard with uid that are saved in organization A", func() {
						var otherOrgId int64 = 2
						cmd := models.SaveDashboardCommand{
							OrgId: otherOrgId,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"uid":   savedDashInFolder.Uid,
								"title": "Dash with existing uid in other org",
							}),
							Overwrite: false,
						}

						res := callSaveWithResult(cmd)
						So(res, ShouldNotBeNil)

						Convey("It should create dashboard in other organization", func() {
							query := models.GetDashboardQuery{OrgId: otherOrgId, Uid: savedDashInFolder.Uid}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.Id, ShouldNotEqual, savedDashInFolder.Id)
							So(query.Result.Id, ShouldEqual, res.Id)
							So(query.Result.OrgId, ShouldEqual, otherOrgId)
							So(query.Result.Uid, ShouldEqual, savedDashInFolder.Uid)
						})
					})
				})
			})

			// Given user has no permission to save

			permissionScenario("Given user has no permission to save", false, func(sc *dashboardPermissionScenarioContext) {

				Convey("When trying to create a new dashboard in the General folder", func() {
					cmd := models.SaveDashboardCommand{
						OrgId: testOrgId,
						Dashboard: simplejson.NewFromAny(map[string]interface{}{
							"title": "Dash",
						}),
						UserId:    10000,
						Overwrite: true,
					}

					err := callSaveWithError(cmd)

					Convey("It should call dashboard guardian with correct arguments", func() {
						So(sc.dashboardGuardianMock.dashId, ShouldEqual, 0)
						So(sc.dashboardGuardianMock.orgId, ShouldEqual, cmd.OrgId)
						So(sc.dashboardGuardianMock.user.UserId, ShouldEqual, cmd.UserId)
					})

					Convey("It should result in access denied error", func() {
						So(err, ShouldNotBeNil)
						So(err, ShouldEqual, models.ErrDashboardUpdateAccessDenied)
					})
				})

				Convey("When trying to create a new dashboard in other folder", func() {
					cmd := models.SaveDashboardCommand{
						OrgId: testOrgId,
						Dashboard: simplejson.NewFromAny(map[string]interface{}{
							"title": "Dash",
						}),
						FolderId:  otherSavedFolder.Id,
						UserId:    10000,
						Overwrite: true,
					}

					err := callSaveWithError(cmd)

					Convey("It should call dashboard guardian with correct arguments", func() {
						So(sc.dashboardGuardianMock.dashId, ShouldEqual, otherSavedFolder.Id)
						So(sc.dashboardGuardianMock.orgId, ShouldEqual, cmd.OrgId)
						So(sc.dashboardGuardianMock.user.UserId, ShouldEqual, cmd.UserId)
					})

					Convey("It should result in access denied error", func() {
						So(err, ShouldNotBeNil)
						So(err, ShouldEqual, models.ErrDashboardUpdateAccessDenied)
					})
				})

				Convey("When trying to update a dashboard by existing id in the General folder", func() {
					cmd := models.SaveDashboardCommand{
						OrgId: testOrgId,
						Dashboard: simplejson.NewFromAny(map[string]interface{}{
							"id":    savedDashInGeneralFolder.Id,
							"title": "Dash",
						}),
						FolderId:  savedDashInGeneralFolder.FolderId,
						UserId:    10000,
						Overwrite: true,
					}

					err := callSaveWithError(cmd)

					Convey("It should call dashboard guardian with correct arguments", func() {
						So(sc.dashboardGuardianMock.dashId, ShouldEqual, savedDashInGeneralFolder.Id)
						So(sc.dashboardGuardianMock.orgId, ShouldEqual, cmd.OrgId)
						So(sc.dashboardGuardianMock.user.UserId, ShouldEqual, cmd.UserId)
					})

					Convey("It should result in access denied error", func() {
						So(err, ShouldNotBeNil)
						So(err, ShouldEqual, models.ErrDashboardUpdateAccessDenied)
					})
				})

				Convey("When trying to update a dashboard by existing id in other folder", func() {
					cmd := models.SaveDashboardCommand{
						OrgId: testOrgId,
						Dashboard: simplejson.NewFromAny(map[string]interface{}{
							"id":    savedDashInFolder.Id,
							"title": "Dash",
						}),
						FolderId:  savedDashInFolder.FolderId,
						UserId:    10000,
						Overwrite: true,
					}

					err := callSaveWithError(cmd)

					Convey("It should call dashboard guardian with correct arguments", func() {
						So(sc.dashboardGuardianMock.dashId, ShouldEqual, savedDashInFolder.Id)
						So(sc.dashboardGuardianMock.orgId, ShouldEqual, cmd.OrgId)
						So(sc.dashboardGuardianMock.user.UserId, ShouldEqual, cmd.UserId)
					})

					Convey("It should result in access denied error", func() {
						So(err, ShouldNotBeNil)
						So(err, ShouldEqual, models.ErrDashboardUpdateAccessDenied)
					})
				})
			})

			// Given user has permission to save

			permissionScenario("Given user has permission to save", true, func(sc *dashboardPermissionScenarioContext) {

				Convey("and overwrite flag is set to false", func() {
					shouldOverwrite := false

					Convey("When creating a dashboard in General folder with same name as dashboard in other folder", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: testOrgId,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    nil,
								"title": savedDashInFolder.Title,
							}),
							FolderId:  0,
							Overwrite: shouldOverwrite,
						}

						res := callSaveWithResult(cmd)
						So(res, ShouldNotBeNil)

						Convey("It should create a new dashboard", func() {
							query := models.GetDashboardQuery{OrgId: cmd.OrgId, Id: res.Id}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.Id, ShouldEqual, res.Id)
							So(query.Result.FolderId, ShouldEqual, 0)
						})
					})

					Convey("When creating a dashboard in other folder with same name as dashboard in General folder", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: testOrgId,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    nil,
								"title": savedDashInGeneralFolder.Title,
							}),
							FolderId:  savedFolder.Id,
							Overwrite: shouldOverwrite,
						}

						res := callSaveWithResult(cmd)
						So(res, ShouldNotBeNil)

						Convey("It should create a new dashboard", func() {
							So(res.Id, ShouldNotEqual, savedDashInGeneralFolder.Id)

							query := models.GetDashboardQuery{OrgId: cmd.OrgId, Id: res.Id}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.FolderId, ShouldEqual, savedFolder.Id)
						})
					})

					Convey("When creating a folder with same name as dashboard in other folder", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: testOrgId,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    nil,
								"title": savedDashInFolder.Title,
							}),
							IsFolder:  true,
							Overwrite: shouldOverwrite,
						}

						res := callSaveWithResult(cmd)
						So(res, ShouldNotBeNil)

						Convey("It should create a new folder", func() {
							So(res.Id, ShouldNotEqual, savedDashInGeneralFolder.Id)
							So(res.IsFolder, ShouldBeTrue)

							query := models.GetDashboardQuery{OrgId: cmd.OrgId, Id: res.Id}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.FolderId, ShouldEqual, 0)
							So(query.Result.IsFolder, ShouldBeTrue)
						})
					})

					Convey("When saving a dashboard without id and uid and unique title in folder", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: testOrgId,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"title": "Dash without id and uid",
							}),
							Overwrite: shouldOverwrite,
						}

						res := callSaveWithResult(cmd)
						So(res, ShouldNotBeNil)

						Convey("It should create a new dashboard", func() {
							So(res.Id, ShouldBeGreaterThan, 0)
							So(len(res.Uid), ShouldBeGreaterThan, 0)
							query := models.GetDashboardQuery{OrgId: cmd.OrgId, Id: res.Id}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.Id, ShouldEqual, res.Id)
							So(query.Result.Uid, ShouldEqual, res.Uid)
						})
					})

					Convey("When saving a dashboard when dashboard id is zero ", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: testOrgId,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    0,
								"title": "Dash with zero id",
							}),
							Overwrite: shouldOverwrite,
						}

						res := callSaveWithResult(cmd)
						So(res, ShouldNotBeNil)

						Convey("It should create a new dashboard", func() {
							query := models.GetDashboardQuery{OrgId: cmd.OrgId, Id: res.Id}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.Id, ShouldEqual, res.Id)
						})
					})

					Convey("When saving a dashboard in non-existing folder", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: testOrgId,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"title": "Expect error",
							}),
							FolderId:  123412321,
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in folder not found error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrFolderNotFound)
						})
					})

					Convey("When updating an existing dashboard by id without current version", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    savedDashInGeneralFolder.Id,
								"title": "test dash 23",
							}),
							FolderId:  savedFolder.Id,
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in version mismatch error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrDashboardVersionMismatch)
						})
					})

					Convey("When updating an existing dashboard by id with current version", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":      savedDashInGeneralFolder.Id,
								"title":   "Updated title",
								"version": savedDashInGeneralFolder.Version,
							}),
							FolderId:  savedFolder.Id,
							Overwrite: shouldOverwrite,
						}

						res := callSaveWithResult(cmd)
						So(res, ShouldNotBeNil)

						Convey("It should update dashboard", func() {
							query := models.GetDashboardQuery{OrgId: cmd.OrgId, Id: savedDashInGeneralFolder.Id}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.Title, ShouldEqual, "Updated title")
							So(query.Result.FolderId, ShouldEqual, savedFolder.Id)
							So(query.Result.Version, ShouldBeGreaterThan, savedDashInGeneralFolder.Version)
						})
					})

					Convey("When updating an existing dashboard by uid without current version", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"uid":   savedDashInFolder.Uid,
								"title": "test dash 23",
							}),
							FolderId:  0,
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in version mismatch error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrDashboardVersionMismatch)
						})
					})

					Convey("When updating an existing dashboard by uid with current version", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"uid":     savedDashInFolder.Uid,
								"title":   "Updated title",
								"version": savedDashInFolder.Version,
							}),
							FolderId:  0,
							Overwrite: shouldOverwrite,
						}

						res := callSaveWithResult(cmd)
						So(res, ShouldNotBeNil)

						Convey("It should update dashboard", func() {
							query := models.GetDashboardQuery{OrgId: cmd.OrgId, Id: savedDashInFolder.Id}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.Title, ShouldEqual, "Updated title")
							So(query.Result.FolderId, ShouldEqual, 0)
							So(query.Result.Version, ShouldBeGreaterThan, savedDashInFolder.Version)
						})
					})

					Convey("When creating a dashboard with same name as dashboard in other folder", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: testOrgId,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    nil,
								"title": savedDashInFolder.Title,
							}),
							FolderId:  savedDashInFolder.FolderId,
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in dashboard with same name in folder error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrDashboardWithSameNameInFolderExists)
						})
					})

					Convey("When creating a dashboard with same name as dashboard in General folder", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: testOrgId,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    nil,
								"title": savedDashInGeneralFolder.Title,
							}),
							FolderId:  savedDashInGeneralFolder.FolderId,
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in dashboard with same name in folder error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrDashboardWithSameNameInFolderExists)
						})
					})

					Convey("When creating a folder with same name as existing folder", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: testOrgId,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    nil,
								"title": savedFolder.Title,
							}),
							IsFolder:  true,
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in dashboard with same name in folder error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrDashboardWithSameNameInFolderExists)
						})
					})
				})

				Convey("and overwrite flag is set to true", func() {
					shouldOverwrite := true

					Convey("When updating an existing dashboard by id without current version", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    savedDashInGeneralFolder.Id,
								"title": "Updated title",
							}),
							FolderId:  savedFolder.Id,
							Overwrite: shouldOverwrite,
						}

						res := callSaveWithResult(cmd)
						So(res, ShouldNotBeNil)

						Convey("It should update dashboard", func() {
							query := models.GetDashboardQuery{OrgId: cmd.OrgId, Id: savedDashInGeneralFolder.Id}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.Title, ShouldEqual, "Updated title")
							So(query.Result.FolderId, ShouldEqual, savedFolder.Id)
							So(query.Result.Version, ShouldBeGreaterThan, savedDashInGeneralFolder.Version)
						})
					})

					Convey("When updating an existing dashboard by uid without current version", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"uid":   savedDashInFolder.Uid,
								"title": "Updated title",
							}),
							FolderId:  0,
							Overwrite: shouldOverwrite,
						}

						res := callSaveWithResult(cmd)
						So(res, ShouldNotBeNil)

						Convey("It should update dashboard", func() {
							query := models.GetDashboardQuery{OrgId: cmd.OrgId, Id: savedDashInFolder.Id}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.Title, ShouldEqual, "Updated title")
							So(query.Result.FolderId, ShouldEqual, 0)
							So(query.Result.Version, ShouldBeGreaterThan, savedDashInFolder.Version)
						})
					})

					Convey("When updating uid for existing dashboard using id", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    savedDashInFolder.Id,
								"uid":   "new-uid",
								"title": savedDashInFolder.Title,
							}),
							Overwrite: shouldOverwrite,
						}

						res := callSaveWithResult(cmd)

						Convey("It should update dashboard", func() {
							So(res, ShouldNotBeNil)
							So(res.Id, ShouldEqual, savedDashInFolder.Id)
							So(res.Uid, ShouldEqual, "new-uid")

							query := models.GetDashboardQuery{OrgId: cmd.OrgId, Id: savedDashInFolder.Id}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.Uid, ShouldEqual, "new-uid")
							So(query.Result.Version, ShouldBeGreaterThan, savedDashInFolder.Version)
						})
					})

					Convey("When updating uid to an existing uid for existing dashboard using id", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    savedDashInFolder.Id,
								"uid":   savedDashInGeneralFolder.Uid,
								"title": savedDashInFolder.Title,
							}),
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in same uid exists error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrDashboardWithSameUIDExists)
						})
					})

					Convey("When creating a dashboard with same name as dashboard in other folder", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: testOrgId,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    nil,
								"title": savedDashInFolder.Title,
							}),
							FolderId:  savedDashInFolder.FolderId,
							Overwrite: shouldOverwrite,
						}

						res := callSaveWithResult(cmd)

						Convey("It should overwrite existing dashboard", func() {
							So(res, ShouldNotBeNil)
							So(res.Id, ShouldEqual, savedDashInFolder.Id)
							So(res.Uid, ShouldEqual, savedDashInFolder.Uid)

							query := models.GetDashboardQuery{OrgId: cmd.OrgId, Id: res.Id}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.Id, ShouldEqual, res.Id)
							So(query.Result.Uid, ShouldEqual, res.Uid)
						})
					})

					Convey("When creating a dashboard with same name as dashboard in General folder", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: testOrgId,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    nil,
								"title": savedDashInGeneralFolder.Title,
							}),
							FolderId:  savedDashInGeneralFolder.FolderId,
							Overwrite: shouldOverwrite,
						}

						res := callSaveWithResult(cmd)

						Convey("It should overwrite existing dashboard", func() {
							So(res, ShouldNotBeNil)
							So(res.Id, ShouldEqual, savedDashInGeneralFolder.Id)
							So(res.Uid, ShouldEqual, savedDashInGeneralFolder.Uid)

							query := models.GetDashboardQuery{OrgId: cmd.OrgId, Id: res.Id}

							err := bus.Dispatch(&query)
							So(err, ShouldBeNil)
							So(query.Result.Id, ShouldEqual, res.Id)
							So(query.Result.Uid, ShouldEqual, res.Uid)
						})
					})

					Convey("When trying to update existing folder to a dashboard using id", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    savedFolder.Id,
								"title": "new title",
							}),
							IsFolder:  false,
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in type mismatch error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrDashboardTypeMismatch)
						})
					})

					Convey("When trying to update existing dashboard to a folder using id", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"id":    savedDashInFolder.Id,
								"title": "new folder title",
							}),
							IsFolder:  true,
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in type mismatch error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrDashboardTypeMismatch)
						})
					})

					Convey("When trying to update existing folder to a dashboard using uid", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"uid":   savedFolder.Uid,
								"title": "new title",
							}),
							IsFolder:  false,
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in type mismatch error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrDashboardTypeMismatch)
						})
					})

					Convey("When trying to update existing dashboard to a folder using uid", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"uid":   savedDashInFolder.Uid,
								"title": "new folder title",
							}),
							IsFolder:  true,
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in type mismatch error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrDashboardTypeMismatch)
						})
					})

					Convey("When trying to update existing folder to a dashboard using title", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"title": savedFolder.Title,
							}),
							IsFolder:  false,
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in dashboard with same name as folder error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrDashboardWithSameNameAsFolder)
						})
					})

					Convey("When trying to update existing dashboard to a folder using title", func() {
						cmd := models.SaveDashboardCommand{
							OrgId: 1,
							Dashboard: simplejson.NewFromAny(map[string]interface{}{
								"title": savedDashInGeneralFolder.Title,
							}),
							IsFolder:  true,
							Overwrite: shouldOverwrite,
						}

						err := callSaveWithError(cmd)

						Convey("It should result in folder with same name as dashboard error", func() {
							So(err, ShouldNotBeNil)
							So(err, ShouldEqual, models.ErrDashboardFolderWithSameNameAsDashboard)
						})
					})

					// Convey("When creating a dashboard with same name as dashboard in General folder", func() {
					// 	cmd := models.SaveDashboardCommand{
					// 		OrgId: testOrgId,
					// 		Dashboard: simplejson.NewFromAny(map[string]interface{}{
					// 			"id":    nil,
					// 			"title": savedDashInGeneralFolder.Title,
					// 		}),
					// 		FolderId:  savedDashInGeneralFolder.FolderId,
					// 		Overwrite: shouldOverwrite,
					// 	}

					// 	err := callSaveWithError(cmd)

					// 	Convey("It should result in dashboard with same name in folder error", func() {
					// 		So(err, ShouldNotBeNil)
					// 		So(err, ShouldEqual, models.ErrDashboardWithSameNameInFolderExists)
					// 	})
					// })

					// Convey("When creating a folder with same name as existing folder", func() {
					// 	cmd := models.SaveDashboardCommand{
					// 		OrgId: testOrgId,
					// 		Dashboard: simplejson.NewFromAny(map[string]interface{}{
					// 			"id":    nil,
					// 			"title": savedFolder.Title,
					// 		}),
					// 		IsFolder:  true,
					// 		Overwrite: shouldOverwrite,
					// 	}

					// 	err := callSaveWithError(cmd)

					// 	Convey("It should result in dashboard with same name in folder error", func() {
					// 		So(err, ShouldNotBeNil)
					// 		So(err, ShouldEqual, models.ErrDashboardWithSameNameInFolderExists)
					// 	})
					// })
				})

				// 	Convey("Should be able to overwrite dashboard in General folder using title", func() {
				// 		dashInGeneral := insertTestDashboard("Dash", 1, 0, false, "prod", "webapp")
				// 		folder := insertTestDashboard("Folder", 1, 0, true, "prod", "webapp")
				// 		insertTestDashboard("Dash", 1, folder.Id, false, "prod", "webapp")

				// 		cmd := models.SaveDashboardCommand{
				// 			OrgId: testOrgId,
				// 			Dashboard: simplejson.NewFromAny(map[string]interface{}{
				// 				"title": "Dash",
				// 			}),
				// 			FolderId:  0,
				// 			Overwrite: true,
				// 		}

				// 		valCmd, err := callValidateDashboardBeforeSave(&cmd)
				// 		So(err, ShouldBeNil)
				// 		So(valCmd.Dashboard.Id, ShouldEqual, dashInGeneral.Id)
				// 		So(valCmd.Dashboard.Uid, ShouldEqual, dashInGeneral.Uid)
				// 	})
			})
		})
	})
}

func mockDashboardGuardian(mock *mockDashboardGuarder) {
	guardian.NewDashboardGuardian = func(dashId int64, orgId int64, user *models.SignedInUser) guardian.IDashboardGuardian {
		mock.orgId = orgId
		mock.dashId = dashId
		mock.user = user
		return mock
	}
}

type mockDashboardGuarder struct {
	dashId                      int64
	orgId                       int64
	user                        *models.SignedInUser
	canSave                     bool
	canSaveCallCounter          int
	canEdit                     bool
	canView                     bool
	canAdmin                    bool
	hasPermission               bool
	checkPermissionBeforeRemove bool
	checkPermissionBeforeUpdate bool
}

func (g *mockDashboardGuarder) CanSave() (bool, error) {
	g.canSaveCallCounter++
	return g.canSave, nil
}

func (g *mockDashboardGuarder) CanEdit() (bool, error) {
	return g.canEdit, nil
}

func (g *mockDashboardGuarder) CanView() (bool, error) {
	return g.canView, nil
}

func (g *mockDashboardGuarder) CanAdmin() (bool, error) {
	return g.canAdmin, nil
}

func (g *mockDashboardGuarder) HasPermission(permission models.PermissionType) (bool, error) {
	return g.hasPermission, nil
}

func (g *mockDashboardGuarder) CheckPermissionBeforeRemove(permission models.PermissionType, aclIdToRemove int64) (bool, error) {
	return g.checkPermissionBeforeRemove, nil
}

func (g *mockDashboardGuarder) CheckPermissionBeforeUpdate(permission models.PermissionType, updatePermissions []*models.DashboardAcl) (bool, error) {
	return g.checkPermissionBeforeUpdate, nil
}

func (g *mockDashboardGuarder) GetAcl() ([]*models.DashboardAclInfoDTO, error) {
	return nil, nil
}

type scenarioContext struct {
	dashboardGuardianMock *mockDashboardGuarder
}

type scenarioFunc func(c *scenarioContext)

func dashboardGuardianScenario(desc string, mock *mockDashboardGuarder, fn scenarioFunc) {
	Convey(desc, func() {
		origNewDashboardGuardian := guardian.NewDashboardGuardian
		mockDashboardGuardian(mock)

		sc := &scenarioContext{
			dashboardGuardianMock: mock,
		}

		defer func() {
			guardian.NewDashboardGuardian = origNewDashboardGuardian
		}()

		fn(sc)
	})
}

type dashboardPermissionScenarioContext struct {
	dashboardGuardianMock *mockDashboardGuarder
}

type dashboardPermissionScenarioFunc func(sc *dashboardPermissionScenarioContext)

func dashboardPermissionScenario(desc string, mock *mockDashboardGuarder, fn dashboardPermissionScenarioFunc) {
	Convey(desc, func() {
		origNewDashboardGuardian := guardian.NewDashboardGuardian
		mockDashboardGuardian(mock)

		sc := &dashboardPermissionScenarioContext{
			dashboardGuardianMock: mock,
		}

		defer func() {
			guardian.NewDashboardGuardian = origNewDashboardGuardian
		}()

		fn(sc)
	})
}

func permissionScenario(desc string, canSave bool, fn dashboardPermissionScenarioFunc) {
	mock := &mockDashboardGuarder{
		canSave: canSave,
	}
	dashboardPermissionScenario(desc, mock, fn)
}

func callSaveWithResult(cmd models.SaveDashboardCommand) *models.Dashboard {
	dto := toSaveDashboardDto(cmd)
	res, _ := NewDashboardService().SaveDashboard(&dto)
	return res
}

func callSaveWithError(cmd models.SaveDashboardCommand) error {
	dto := toSaveDashboardDto(cmd)
	_, err := NewDashboardService().SaveDashboard(&dto)
	return err
}

func dashboardServiceScenario(desc string, mock *mockDashboardGuarder, fn scenarioFunc) {
	Convey(desc, func() {
		origNewDashboardGuardian := guardian.NewDashboardGuardian
		mockDashboardGuardian(mock)

		sc := &scenarioContext{
			dashboardGuardianMock: mock,
		}

		defer func() {
			guardian.NewDashboardGuardian = origNewDashboardGuardian
		}()

		fn(sc)
	})
}

func insertTestDashboard(title string, orgId int64, folderId int64) *models.Dashboard {
	cmd := models.SaveDashboardCommand{
		OrgId:    orgId,
		FolderId: folderId,
		IsFolder: false,
		Dashboard: simplejson.NewFromAny(map[string]interface{}{
			"id":    nil,
			"title": title,
		}),
	}

	dto := SaveDashboardDTO{
		OrgId:     orgId,
		Dashboard: cmd.GetDashboardModel(),
		User: &models.SignedInUser{
			UserId:  1,
			OrgRole: models.ROLE_ADMIN,
		},
	}

	res, err := NewDashboardService().SaveDashboard(&dto)
	So(err, ShouldBeNil)

	return res
}

func insertTestFolder(title string, orgId int64) *models.Dashboard {
	cmd := models.SaveDashboardCommand{
		OrgId:    orgId,
		FolderId: 0,
		IsFolder: true,
		Dashboard: simplejson.NewFromAny(map[string]interface{}{
			"id":    nil,
			"title": title,
		}),
	}

	dto := SaveDashboardDTO{
		OrgId:     orgId,
		Dashboard: cmd.GetDashboardModel(),
		User: &models.SignedInUser{
			UserId:  1,
			OrgRole: models.ROLE_ADMIN,
		},
	}

	res, err := NewDashboardService().SaveDashboard(&dto)
	So(err, ShouldBeNil)

	return res
}

func toSaveDashboardDto(cmd models.SaveDashboardCommand) SaveDashboardDTO {
	dash := (&cmd).GetDashboardModel()

	return SaveDashboardDTO{
		Dashboard: dash,
		Message:   cmd.Message,
		OrgId:     cmd.OrgId,
		User:      &models.SignedInUser{UserId: cmd.UserId},
		Overwrite: cmd.Overwrite,
	}
}
