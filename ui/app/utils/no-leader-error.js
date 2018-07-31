import { AdapterError } from 'ember-data/adapters/errors';

export const NO_LEADER = 'No cluster leader';

const NoLeaderError = function() {
  AdapterError.call(this, [], NO_LEADER);
};

NoLeaderError.prototype = Object.create(AdapterError.prototype);

export default NoLeaderError;
