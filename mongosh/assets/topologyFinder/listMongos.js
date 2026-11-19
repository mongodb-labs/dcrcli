// Routers that have checked in recently. getShardMap lists shards and config
// servers, not mongos. A mongos pings config.mongos about every 30 seconds;
// entries older than 10 minutes are left out so a stopped router does not
// fail the cluster health check.
var cutoff = new Date(Date.now() - 10 * 60 * 1000);
db.getSiblingDB("config").mongos.find({ ping: { $gte: cutoff } }, { _id: 1 }).toArray().map(function (d) {
  return d._id;
});
