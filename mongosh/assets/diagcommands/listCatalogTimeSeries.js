try {
  const maxCollections = (typeof _maxCollections !== "undefined") ? _maxCollections : 2500;
  const adminDB = db.getSiblingDB('admin');
  const fetched = adminDB.aggregate([
    { $listCatalog: {} },
    { $match: { name: { $regex: '^system\\.buckets\\.' } } },
    { $limit: maxCollections + 1 }
  ]).toArray();
  const truncated = fetched.length > maxCollections;
  const buckets = truncated ? fetched.slice(0, maxCollections) : fetched;
  printjson({
    buckets: buckets,
    truncated: truncated,
    maxCollections: maxCollections
  });
} catch (e) {
  print("ERROR: " + e);
}
