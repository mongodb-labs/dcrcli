(function () {
  try {
    var parsed = db.adminCommand({ getCmdLineOpts: 1 }).parsed;
    if (parsed && parsed.storage && parsed.storage.dbPath) {
      return parsed.storage.dbPath;
    }
  } catch (e) {}
  return "";
})()
