import React from 'react';
import UsersTable from '../../components/table/users';

const ProviderUsersPage = () => {
  return (
    <div className='px-2'>
      <UsersTable apiPrefix='/api/provider/users' providerMode />
    </div>
  );
};

export default ProviderUsersPage;
