// Copyright 2013 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package checkers_test

import (
	"fmt"
	"testing"
	"time"

	gc "gopkg.in/check.v1"

	jc "github.com/juju/testing/checkers"
)

func Test(t *testing.T) { gc.TestingT(t) }

type CheckerSuite struct{}

var _ = gc.Suite(&CheckerSuite{})

func (s *CheckerSuite) TestHasPrefix(c *gc.C) {
	c.Assert("foo bar", jc.HasPrefix, "foo")
	c.Assert("foo bar", gc.Not(jc.HasPrefix), "omg")
}

func (s *CheckerSuite) TestHasSuffix(c *gc.C) {
	c.Assert("foo bar", jc.HasSuffix, "bar")
	c.Assert("foo bar", gc.Not(jc.HasSuffix), "omg")
}

func (s *CheckerSuite) TestContains(c *gc.C) {
	c.Assert("foo bar baz", jc.Contains, "foo")
	c.Assert("foo bar baz", jc.Contains, "bar")
	c.Assert("foo bar baz", jc.Contains, "baz")
	c.Assert("foo bar baz", gc.Not(jc.Contains), "omg")
}

func (s *CheckerSuite) TestTimeBetween(c *gc.C) {
	now := time.Now()
	earlier := now.Add(-1 * time.Second)
	later := now.Add(time.Second)

	checkOK := func(value interface{}, start, end time.Time) {
		checker := jc.TimeBetween(start, end)
		value, msg := checker.Check([]interface{}{value}, nil)
		c.Check(value, jc.IsTrue)
		c.Check(msg, gc.Equals, "")
	}

	checkFails := func(value interface{}, start, end time.Time, match string) {
		checker := jc.TimeBetween(start, end)
		value, msg := checker.Check([]interface{}{value}, nil)
		c.Check(value, jc.IsFalse)
		c.Check(msg, gc.Matches, match)
	}

	checkOK(now, earlier, later)
	// Later can be before earlier...
	checkOK(now, later, earlier)
	// check at bounds
	checkOK(earlier, earlier, later)
	checkOK(later, earlier, later)

	checkFails(earlier, now, later, `obtained time .* is before start time .*`)
	checkFails(later, now, earlier, `obtained time .* is after end time .*`)
	checkFails(42, now, earlier, `obtained value type must be time.Time`)
}

func (s *CheckerSuite) TestSameContents(c *gc.C) {
	//// positive cases ////

	// same
	c.Check(
		[]int{1, 2, 3}, jc.SameContents,
		[]int{1, 2, 3})

	// empty
	c.Check(
		[]int{}, jc.SameContents,
		[]int{})

	// single
	c.Check(
		[]int{1}, jc.SameContents,
		[]int{1})

	// different order
	c.Check(
		[]int{1, 2, 3}, jc.SameContents,
		[]int{3, 2, 1})

	// multiple copies of same
	c.Check(
		[]int{1, 1, 2}, jc.SameContents,
		[]int{2, 1, 1})

	type test struct {
		s string
		i int
	}

	// test structs
	c.Check(
		[]test{{"a", 1}, {"b", 2}}, jc.SameContents,
		[]test{{"b", 2}, {"a", 1}})

	//// negative cases ////

	// different contents
	c.Check(
		[]int{1, 3, 2, 5}, gc.Not(jc.SameContents),
		[]int{5, 2, 3, 4})

	// different size slices
	c.Check(
		[]int{1, 2, 3}, gc.Not(jc.SameContents),
		[]int{1, 2})

	// different counts of same items
	c.Check(
		[]int{1, 1, 2}, gc.Not(jc.SameContents),
		[]int{1, 2, 2})

	/// Error cases ///
	//  note: for these tests, we can't use gc.Not, since Not passes the error value through
	// and checks with a non-empty error always count as failed
	// Oddly, there doesn't seem to actually be a way to check for an error from a Checker.

	// different type
	res, err := jc.SameContents.Check([]interface{}{
		[]string{"1", "2"},
		[]int{1, 2},
	}, []string{})
	c.Check(res, jc.IsFalse)
	c.Check(err, gc.Not(gc.Equals), "")

	// obtained not a slice
	res, err = jc.SameContents.Check([]interface{}{
		"test",
		[]int{1},
	}, []string{})
	c.Check(res, jc.IsFalse)
	c.Check(err, gc.Not(gc.Equals), "")

	// expected not a slice
	res, err = jc.SameContents.Check([]interface{}{
		[]int{1},
		"test",
	}, []string{})
	c.Check(res, jc.IsFalse)
	c.Check(err, gc.Not(gc.Equals), "")
}

type stack_error struct {
	message string
	stack   []string
}

func (s *stack_error) Error() string {
	return s.message
}
func (s *stack_error) StackTrace() []string {
	return s.stack
}

func (s *CheckerSuite) TestIsNil(c *gc.C) {
	checkOK := func(value interface{}) {
		value, msg := jc.IsNil.Check([]interface{}{value}, nil)
		c.Check(value, jc.IsTrue)
		c.Check(msg, gc.Equals, "")
	}

	checkFails := func(value interface{}, match string) {
		value, msg := jc.IsNil.Check([]interface{}{value}, nil)
		c.Check(value, jc.IsFalse)
		c.Check(msg, gc.Matches, match)
	}

	var typedNil *stack_error
	checkOK(nil)
	checkOK(typedNil)

	checkFails(fmt.Errorf("an error"), "")

	emptyStack := &stack_error{"message", nil}
	checkFails(emptyStack, "")

	withStack := &stack_error{"message", []string{
		"filename:line", "filename2:line2"}}
	checkFails(withStack, "error stack:\n\tfilename:line\n\tfilename2:line2")
}
