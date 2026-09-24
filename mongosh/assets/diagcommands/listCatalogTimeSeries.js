try {
  const skip = { admin: 1, local: 1, config: 1 };
  const listed = db.adminCommand({ listDatabases: 1, nameOnly: true });
  if (!listed || listed.ok !== 1) {
    throw (listed && (listed.errmsg || listed.codeName)) || "listDatabases failed";
  }
  const names = (listed.databases || []).map(function (d) { return d.name; });
  const buckets = [];
  const errors = [];
  names.forEach(function (name) {
    if (skip[name]) {
      return;
    }
    try {
      const hits = db.getSiblingDB(name).aggregate([
        { $listCatalog: {} },
        { $match: { name: { $regex: '^system\\.buckets\\.' } } }
      ]).toArray();
      hits.forEach(function (doc) {
        doc.db = name;
        buckets.push(doc);
      });
    } catch (e) {
      errors.push({ db: name, error: '' + e });
    }
  });
  printjson({ buckets: buckets, errors: errors });
  if (errors.length) {
    print("ERROR: " + errors.length + " database(s) failed");
  }
} catch (e) {
  print("ERROR: " + e);
}
