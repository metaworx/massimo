- CI's Android jobs keep a cache each of Gradle's downloads and its
  wrapper, so that a run no longer fetches them all from Maven Central,
  which refused one with 429 Too Many Requests (AP SteadyTests).
