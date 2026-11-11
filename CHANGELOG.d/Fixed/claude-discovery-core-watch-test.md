- The settings file watch's test writes each content whole, renamed into
  place: a busy runner paused between truncating the file and writing it,
  and the watch saw two changes where the test made one.
