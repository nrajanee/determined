//go:build integration
// +build integration

package internal

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/determined-ai/determined/master/pkg/model"
	"github.com/determined-ai/determined/proto/pkg/apiv1"
	"github.com/determined-ai/determined/proto/pkg/commonv1"
	"github.com/determined-ai/determined/proto/pkg/trialv1"
)

func errTrialNotFound(id int) error {
	return status.Errorf(codes.NotFound, "trial %d not found", id)
}

func createTestTrial(
	t *testing.T, api *apiServer, curUser model.User,
) *model.Trial {
	exp := createTestExpWithProjectID(t, api, curUser, 1)

	task := &model.Task{
		TaskType: model.TaskTypeTrial,
		TaskID:   trialTaskID(exp.ID, model.NewRequestID(rand.Reader)),
	}
	require.NoError(t, api.m.db.AddTask(task))

	trial := &model.Trial{
		StartTime:    time.Now(),
		State:        model.PausedState,
		ExperimentID: exp.ID,
		TaskID:       task.TaskID,
	}
	require.NoError(t, api.m.db.AddTrial(trial))

	// Return trial exactly the way the API will generally get it.
	outTrial, err := api.m.db.TrialByID(trial.ID)
	require.NoError(t, err)
	return outTrial
}

func createTestTrialWithMetrics(
	ctx context.Context, t *testing.T, api *apiServer, curUser model.User, includeBatchMetrics bool,
) (*model.Trial, []*commonv1.Metrics, []*commonv1.Metrics) {
	var trainingMetrics, validationMetrics []*commonv1.Metrics
	trial := createTestTrial(t, api, curUser)

	for i := 0; i < 10; i++ {
		trainMetrics := &commonv1.Metrics{
			AvgMetrics: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"epoch": {
						Kind: &structpb.Value_NumberValue{
							NumberValue: float64(i),
						},
					},
					"loss": {
						Kind: &structpb.Value_NumberValue{
							NumberValue: float64(i),
						},
					},

					"loss2": {
						Kind: &structpb.Value_NumberValue{
							NumberValue: float64(i),
						},
					},
					"textMetric": {
						Kind: &structpb.Value_StringValue{
							StringValue: "random_text",
						},
					},
				},
			},
		}
		if includeBatchMetrics {
			trainMetrics.BatchMetrics = []*structpb.Struct{
				{
					Fields: map[string]*structpb.Value{
						"batch_loss": {
							Kind: &structpb.Value_NumberValue{
								NumberValue: float64(i),
							},
						},
					},
				},
			}
		}

		_, err := api.ReportTrialTrainingMetrics(ctx,
			&apiv1.ReportTrialTrainingMetricsRequest{
				TrainingMetrics: &trialv1.TrialMetrics{
					TrialId:        int32(trial.ID),
					TrialRunId:     0,
					StepsCompleted: int32(i),
					Metrics:        trainMetrics,
				},
			})
		require.NoError(t, err)
		trainingMetrics = append(trainingMetrics, trainMetrics)

		valMetrics := &commonv1.Metrics{
			AvgMetrics: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"epoch": {
						Kind: &structpb.Value_NumberValue{
							NumberValue: float64(i),
						},
					},
					"val_loss": {
						Kind: &structpb.Value_NumberValue{
							NumberValue: float64(i),
						},
					},

					"val_loss2": {
						Kind: &structpb.Value_NumberValue{
							NumberValue: float64(i),
						},
					},
					"textMetric": {
						Kind: &structpb.Value_StringValue{
							StringValue: "random_text",
						},
					},
				},
			},
		}
		_, err = api.ReportTrialValidationMetrics(ctx,
			&apiv1.ReportTrialValidationMetricsRequest{
				ValidationMetrics: &trialv1.TrialMetrics{
					TrialId:        int32(trial.ID),
					TrialRunId:     0,
					StepsCompleted: int32(i),
					Metrics:        valMetrics,
				},
			})
		require.NoError(t, err)
		validationMetrics = append(validationMetrics, valMetrics)
	}

	return trial, trainingMetrics, validationMetrics
}

func compareMetrics(
	t *testing.T, trialIDs []int,
	resp []*trialv1.MetricsReport, expected []*commonv1.Metrics, isValidation bool,
) {
	require.NotNil(t, resp)

	trialIndex := 0
	totalBatches := 0
	for i, actual := range resp {
		if i != 0 && i%(len(expected)/len(trialIDs)) == 0 {
			trialIndex++
			totalBatches = 0
		}

		metrics := map[string]any{
			"avg_metrics":   expected[i].AvgMetrics.AsMap(),
			"batch_metrics": nil,
		}
		if expected[i].BatchMetrics != nil {
			var batchMetrics []any
			for _, b := range expected[i].BatchMetrics {
				batchMetrics = append(batchMetrics, b.AsMap())
			}
			metrics["batch_metrics"] = batchMetrics
		}
		if isValidation {
			metrics = map[string]any{
				"validation_metrics": expected[i].AvgMetrics.AsMap(),
			}
		}
		protoStruct, err := structpb.NewStruct(metrics)
		require.NoError(t, err)

		expectedRow := &trialv1.MetricsReport{
			TrialId:      int32(trialIDs[trialIndex]),
			EndTime:      actual.EndTime,
			Metrics:      protoStruct,
			TotalBatches: int32(totalBatches),
			TrialRunId:   int32(0),
			Id:           actual.Id,
		}
		proto.Equal(actual, expectedRow)
		require.Equal(t, actual, expectedRow)

		totalBatches++
	}
}

func isMultiTrialSampleCorrect(expectedMetrics []*commonv1.Metrics,
	actualMetrics *apiv1.DownsampledMetrics) bool {
	// Checking if metric names and their values are equal.
	for i := 0; i < len(actualMetrics.Data); i++ {
		allActualAvgMetrics := actualMetrics.Data
		epoch := int(*allActualAvgMetrics[i].Epoch)
		// use epoch to match because in downsampling returned values are randomized.
		expectedAvgMetrics := expectedMetrics[epoch].AvgMetrics.AsMap()
		for metricName := range expectedAvgMetrics {
			switch expectedAvgMetrics[metricName].(type) { //nolint:gocritic
			case float64:
				actualAvgMetrics := allActualAvgMetrics[i].Values.AsMap()
				expectedVal := expectedAvgMetrics[metricName].(float64)
				if metricName == "epoch" {
					if expectedVal != float64(*allActualAvgMetrics[i].Epoch) {
						return false
					}
					continue
				}
				if actualAvgMetrics[metricName] == nil {
					return false
				}
				actualVal := actualAvgMetrics[metricName].(float64)
				if expectedVal != actualVal {
					return false
				}
			default:
				continue // non-float values are not handled in API
			}
		}
	}
	return true
}

func TestMultiTrialSampleMetrics(t *testing.T) {
	api, curUser, ctx := setupAPITest(t, nil)

	trial, expectedTrainMetrics, expectedValMetrics := createTestTrialWithMetrics(
		ctx, t, api, curUser, false)

	var trainMetricNames []string
	var metricIds []string
	for metricName := range expectedTrainMetrics[0].AvgMetrics.AsMap() {
		if metricName == "textMetric" { //nolint:goconst
			continue
		}
		trainMetricNames = append(trainMetricNames, metricName)
		metricIds = append(metricIds, "training."+metricName)
	}

	maxDataPoints := 7
	actualTrainingMetrics, err := api.MultiTrialSample(int32(trial.ID), trainMetricNames,
		apiv1.MetricType_METRIC_TYPE_TRAINING, maxDataPoints, 0, 10, false, nil, []string{})
	require.NoError(t, err)
	require.Equal(t, 1, len(actualTrainingMetrics))
	var validationMetricNames []string
	for metricName := range expectedValMetrics[0].AvgMetrics.AsMap() {
		if metricName == "textMetric" {
			continue
		}
		validationMetricNames = append(validationMetricNames, metricName)
		metricIds = append(metricIds, "validation."+metricName)
	}

	actualValidationTrainingMetrics, err := api.MultiTrialSample(int32(trial.ID),
		validationMetricNames, apiv1.MetricType_METRIC_TYPE_VALIDATION, maxDataPoints,
		0, 10, false, nil, []string{})
	require.Equal(t, 1, len(actualValidationTrainingMetrics))
	require.NoError(t, err)
	require.True(t, isMultiTrialSampleCorrect(expectedTrainMetrics, actualTrainingMetrics[0]))
	require.True(t, isMultiTrialSampleCorrect(expectedValMetrics, actualValidationTrainingMetrics[0]))

	actualAllMetrics, err := api.MultiTrialSample(int32(trial.ID), []string{},
		apiv1.MetricType_METRIC_TYPE_UNSPECIFIED, maxDataPoints, 0, 10, false, nil, metricIds)
	require.Equal(t, 2, len(actualAllMetrics))
	require.NoError(t, err)
	require.Equal(t, maxDataPoints, len(actualAllMetrics[0].Data)) // max datapoints check
	require.Equal(t, maxDataPoints, len(actualAllMetrics[1].Data)) // max datapoints check
	require.True(t, isMultiTrialSampleCorrect(expectedTrainMetrics, actualAllMetrics[0]))
	require.True(t, isMultiTrialSampleCorrect(expectedValMetrics, actualAllMetrics[1]))
}
func TestStreamTrainingMetrics(t *testing.T) {
	api, curUser, ctx := setupAPITest(t, nil)

	var trials []*model.Trial
	var trainingMetrics, validationMetrics [][]*commonv1.Metrics
	for _, haveBatchMetrics := range []bool{false, true} {
		trial, trainMetrics, valMetrics := createTestTrialWithMetrics(
			ctx, t, api, curUser, haveBatchMetrics)
		trials = append(trials, trial)
		trainingMetrics = append(trainingMetrics, trainMetrics)
		validationMetrics = append(validationMetrics, valMetrics)
	}

	cases := []struct {
		requestFunc  func(trialIDs []int32) ([]*trialv1.MetricsReport, error)
		metrics      [][]*commonv1.Metrics
		isValidation bool
	}{
		{
			func(trialIDs []int32) ([]*trialv1.MetricsReport, error) {
				res := &mockStream[*apiv1.GetTrainingMetricsResponse]{ctx: ctx}
				err := api.GetTrainingMetrics(&apiv1.GetTrainingMetricsRequest{
					TrialIds: trialIDs,
				}, res)
				if err != nil {
					return nil, err
				}
				var out []*trialv1.MetricsReport
				for _, d := range res.data {
					out = append(out, d.Metrics...)
				}
				return out, nil
			}, trainingMetrics, false,
		},
		{
			func(trialIDs []int32) ([]*trialv1.MetricsReport, error) {
				res := &mockStream[*apiv1.GetValidationMetricsResponse]{ctx: ctx}
				err := api.GetValidationMetrics(&apiv1.GetValidationMetricsRequest{
					TrialIds: trialIDs,
				}, res)
				if err != nil {
					return nil, err
				}
				var out []*trialv1.MetricsReport
				for _, d := range res.data {
					out = append(out, d.Metrics...)
				}
				return out, nil
			}, validationMetrics, true,
		},
	}
	for _, curCase := range cases {
		// No trial IDs.
		_, err := curCase.requestFunc([]int32{})
		require.Error(t, err)
		require.Equal(t, status.Code(err), codes.InvalidArgument)

		// Trial IDs not found.
		_, err = curCase.requestFunc([]int32{-1})
		require.Equal(t, status.Code(err), codes.NotFound)

		// One trial.
		resp, err := curCase.requestFunc([]int32{int32(trials[0].ID)})
		require.NoError(t, err)
		compareMetrics(t, []int{trials[0].ID}, resp, curCase.metrics[0], curCase.isValidation)

		// Other trial.
		resp, err = curCase.requestFunc([]int32{int32(trials[1].ID)})
		require.NoError(t, err)
		compareMetrics(t, []int{trials[1].ID}, resp, curCase.metrics[1], curCase.isValidation)

		// Both trials.
		resp, err = curCase.requestFunc([]int32{int32(trials[1].ID), int32(trials[0].ID)})
		require.NoError(t, err)
		compareMetrics(t, []int{trials[0].ID, trials[1].ID}, resp,
			append(curCase.metrics[0], curCase.metrics[1]...), curCase.isValidation)
	}
}

func TestTrialAuthZ(t *testing.T) {
	api, authZExp, _, curUser, ctx := setupExpAuthTest(t, nil)
	trial := createTestTrial(t, api, curUser)

	cases := []struct {
		DenyFuncName   string
		IDToReqCall    func(id int) error
		SkipActionFunc bool
	}{
		{"CanGetExperimentArtifacts", func(id int) error {
			return api.TrialLogs(&apiv1.TrialLogsRequest{
				TrialId: int32(id),
			}, &mockStream[*apiv1.TrialLogsResponse]{ctx: ctx})
		}, false},
		{"CanGetExperimentArtifacts", func(id int) error {
			return api.TrialLogsFields(&apiv1.TrialLogsFieldsRequest{
				TrialId: int32(id),
			}, &mockStream[*apiv1.TrialLogsFieldsResponse]{ctx: ctx})
		}, false},
		{"CanGetExperimentArtifacts", func(id int) error {
			_, err := api.GetTrialCheckpoints(ctx, &apiv1.GetTrialCheckpointsRequest{
				Id: int32(id),
			})
			return err
		}, false},
		{"CanEditExperiment", func(id int) error {
			_, err := api.KillTrial(ctx, &apiv1.KillTrialRequest{
				Id: int32(id),
			})
			return err
		}, false},
		{"CanGetExperimentArtifacts", func(id int) error {
			_, err := api.GetTrial(ctx, &apiv1.GetTrialRequest{
				TrialId: int32(id),
			})
			return err
		}, true},
		{"CanGetExperimentArtifacts", func(id int) error {
			_, err := api.SummarizeTrial(ctx, &apiv1.SummarizeTrialRequest{
				TrialId: int32(id),
			})
			return err
		}, false},
		{"CanGetExperimentArtifacts", func(id int) error {
			_, err := api.CompareTrials(ctx, &apiv1.CompareTrialsRequest{
				TrialIds: []int32{int32(id)},
			})
			return err
		}, false},
		{"CanGetExperimentArtifacts", func(id int) error {
			_, err := api.GetTrialWorkloads(ctx, &apiv1.GetTrialWorkloadsRequest{
				TrialId: int32(id),
			})
			return err
		}, false},
		{"CanGetExperimentArtifacts", func(id int) error {
			return api.GetTrialProfilerMetrics(&apiv1.GetTrialProfilerMetricsRequest{
				Labels: &trialv1.TrialProfilerMetricLabels{TrialId: int32(id)},
			}, &mockStream[*apiv1.GetTrialProfilerMetricsResponse]{ctx: ctx})
		}, false},
		{"CanGetExperimentArtifacts", func(id int) error {
			return api.GetTrialProfilerAvailableSeries(
				&apiv1.GetTrialProfilerAvailableSeriesRequest{
					TrialId: int32(id),
				}, &mockStream[*apiv1.GetTrialProfilerAvailableSeriesResponse]{ctx: ctx})
		}, false},
		{"CanEditExperiment", func(id int) error {
			_, err := api.PostTrialProfilerMetricsBatch(ctx,
				&apiv1.PostTrialProfilerMetricsBatchRequest{
					Batches: []*trialv1.TrialProfilerMetricsBatch{
						{
							Labels: &trialv1.TrialProfilerMetricLabels{TrialId: int32(id)},
						},
					},
				})
			return err
		}, false},
		{"CanGetExperimentArtifacts", func(id int) error {
			_, err := api.GetCurrentTrialSearcherOperation(ctx,
				&apiv1.GetCurrentTrialSearcherOperationRequest{
					TrialId: int32(id),
				})
			return err
		}, false},
		{"CanEditExperiment", func(id int) error {
			_, err := api.CompleteTrialSearcherValidation(ctx,
				&apiv1.CompleteTrialSearcherValidationRequest{
					TrialId: int32(id),
				})
			return err
		}, false},
		{"CanEditExperiment", func(id int) error {
			_, err := api.ReportTrialSearcherEarlyExit(ctx,
				&apiv1.ReportTrialSearcherEarlyExitRequest{
					TrialId: int32(id),
				})
			return err
		}, false},
		{"CanEditExperiment", func(id int) error {
			_, err := api.ReportTrialProgress(ctx,
				&apiv1.ReportTrialProgressRequest{
					TrialId: int32(id),
				})
			return err
		}, false},
		{"CanEditExperiment", func(id int) error {
			_, err := api.ReportTrialTrainingMetrics(ctx,
				&apiv1.ReportTrialTrainingMetricsRequest{
					TrainingMetrics: &trialv1.TrialMetrics{TrialId: int32(id)},
				})
			return err
		}, false},
		{"CanEditExperiment", func(id int) error {
			_, err := api.ReportTrialValidationMetrics(ctx,
				&apiv1.ReportTrialValidationMetricsRequest{
					ValidationMetrics: &trialv1.TrialMetrics{TrialId: int32(id)},
				})
			return err
		}, false},
		{"CanEditExperiment", func(id int) error {
			_, err := api.PostTrialRunnerMetadata(ctx, &apiv1.PostTrialRunnerMetadataRequest{
				TrialId: int32(id),
			})
			return err
		}, false},
		{"CanGetExperimentArtifacts", func(id int) error {
			_, err := api.LaunchTensorboard(ctx, &apiv1.LaunchTensorboardRequest{
				TrialIds: []int32{int32(id)},
			})
			return err
		}, false},
	}

	for _, curCase := range cases {
		require.ErrorIs(t, curCase.IDToReqCall(-999), errTrialNotFound(-999))

		// Can't view trials experiment gives same error.
		authZExp.On("CanGetExperiment", mock.Anything, curUser, mock.Anything).
			Return(false, nil).Once()
		require.ErrorIs(t, curCase.IDToReqCall(trial.ID), errTrialNotFound(trial.ID))

		// Experiment view error returns error unmodified.
		expectedErr := fmt.Errorf("canGetTrialError")
		authZExp.On("CanGetExperiment", mock.Anything, curUser, mock.Anything).
			Return(false, expectedErr).Once()
		require.ErrorIs(t, curCase.IDToReqCall(trial.ID), expectedErr)

		// Action func error returns error in forbidden.
		expectedErr = status.Error(codes.PermissionDenied, curCase.DenyFuncName+"Error")
		authZExp.On("CanGetExperiment", mock.Anything, curUser, mock.Anything).
			Return(true, nil).Once()
		authZExp.On(curCase.DenyFuncName, mock.Anything, curUser, mock.Anything).
			Return(fmt.Errorf(curCase.DenyFuncName + "Error")).Once()
		require.ErrorIs(t, curCase.IDToReqCall(trial.ID), expectedErr)
	}
}

func compareTrialsResponseToBatches(resp *apiv1.CompareTrialsResponse) []int32 {
	compTrial := resp.Trials[0]
	compMetrics := compTrial.Metrics[0]

	sampleBatches := []int32{}

	for _, m := range compMetrics.Data {
		sampleBatches = append(sampleBatches, m.Batches)
	}

	return sampleBatches
}

func TestCompareTrialsSampling(t *testing.T) {
	api, curUser, ctx := setupAPITest(t, nil)

	trial, _, _ := createTestTrialWithMetrics(
		ctx, t, api, curUser, false)

	const DATAPOINTS = 3

	req := &apiv1.CompareTrialsRequest{
		TrialIds:      []int32{int32(trial.ID)},
		MaxDatapoints: DATAPOINTS,
		MetricNames:   []string{"loss"},
		StartBatches:  0,
		EndBatches:    1000,
		MetricType:    apiv1.MetricType_METRIC_TYPE_TRAINING,
		Scale:         apiv1.Scale_SCALE_LINEAR,
	}

	resp, err := api.CompareTrials(ctx, req)
	require.NoError(t, err)

	sampleBatches1 := compareTrialsResponseToBatches(resp)
	require.Equal(t, DATAPOINTS, len(sampleBatches1))

	resp, err = api.CompareTrials(ctx, req)
	require.NoError(t, err)

	sampleBatches2 := compareTrialsResponseToBatches(resp)

	require.Equal(t, sampleBatches1, sampleBatches2)
}
