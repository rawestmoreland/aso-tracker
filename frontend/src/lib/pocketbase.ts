import PocketBase from 'pocketbase';

const PB_URL = process.env.PB_URL || 'http://localhost:8080';

const pb = new PocketBase(PB_URL);

export default pb;
