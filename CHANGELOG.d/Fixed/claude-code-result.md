- A change to documents alone on code that failed no longer passes CI:
  the job Code result carries the verdict of the last run that tested
  the code, so the nightly build cannot release it.
