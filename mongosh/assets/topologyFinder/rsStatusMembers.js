(function () {
  try {
    var s = rs.status();
    return s.members.map(function (m) {
      return { name: m.name, stateStr: m.stateStr };
    });
  } catch (e) {
    return { error: true, message: String(e.message || e) };
  }
})()
