import PocketBase from 'pocketbase';

const PB_URL = import.meta.env.VITE_PB_URL || 'http://localhost:8080';

const pb = new PocketBase(PB_URL);

export default pb;
