package concurrency

import (
	"context"
	"fmt"
	"golang.org/x/sync/semaphore"
)

type GoRoutineRunner[T any] struct {
	jobs              []Job[T]
	maxConcurrentJobs int
	clearJobsAfterRun bool
}

type Job[T any] func(index int) (T, error)

const defaultMaxConcurrentJobs = 1000

func NewGoRoutineRunner[T any]() *GoRoutineRunner[T] {
	return &GoRoutineRunner[T]{
		jobs:              make([]Job[T], 0),
		maxConcurrentJobs: defaultMaxConcurrentJobs,
	}
}

func (r *GoRoutineRunner[T]) SetMaxConcurrentJobs(maxConcurrentJobs int) *GoRoutineRunner[T] {
	if maxConcurrentJobs == 0 {
		maxConcurrentJobs = defaultMaxConcurrentJobs
	}
	r.maxConcurrentJobs = maxConcurrentJobs
	return r
}

func (r *GoRoutineRunner[T]) SetClearJobsAfterRun(clearJobsAfterRun bool) *GoRoutineRunner[T] {
	r.clearJobsAfterRun = clearJobsAfterRun
	return r
}

func (r *GoRoutineRunner[T]) AddJob(jobs ...Job[T]) *GoRoutineRunner[T] {
	r.jobs = append(r.jobs, jobs...)
	return r
}

func (r *GoRoutineRunner[T]) Run(ctx context.Context) ([]T, []error, error) {
	if len(r.jobs) == 0 {
		return nil, nil, fmt.Errorf("no jobs to run")
	}

	results := make([]T, len(r.jobs))
	errors := make([]error, len(r.jobs))

	maxConcurrentJobs := r.maxConcurrentJobs

	if r.maxConcurrentJobs <= 0 {
		// No limit
		maxConcurrentJobs = len(r.jobs)
	}

	sem := semaphore.NewWeighted(int64(maxConcurrentJobs))

	for jobIndex, job := range r.jobs {
		if err := sem.Acquire(ctx, 1); err != nil {
			return nil, nil, err
		}

		go func(resultIndex int, job Job[T]) {
			defer sem.Release(1)
			result, err := job(resultIndex)
			results[resultIndex] = result
			errors[resultIndex] = err
		}(jobIndex, job)
	}

	if err := sem.Acquire(ctx, int64(maxConcurrentJobs)); err != nil {
		return nil, nil, err
	}

	if r.clearJobsAfterRun {
		r.jobs = make([]Job[T], 0)
	}

	return results, errors, nil
}
